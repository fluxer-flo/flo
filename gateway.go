package flo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

//go:generate stringer -type=GatewayOpcode -output=gateway_string.go

// Gateway manages one or more [Shard]s and provides a high-level interface for listening to events.
// Sharding is supported, but at the time of writing Fluxer does not support it yet - it has instead been tested with Discord.
type Gateway struct {
	// Auth specifies the token to send when connecting.
	Auth string
	// Cache specifies the caching target. If nil is specified, nothing is cached.
	Cache *Cache
	// ConnURL specifies the initial URL for establishing a connection.
	ConnURL *url.URL
	// Dialer specifies options for connecting to the WebSocket server (from Gorilla WebSocket).
	// If nil, websocket.DefaultDialer is used.
	Dialer *websocket.Dialer
	// ReconnectDelay determines how long to wait before reconnecting on the nth dial attempt.
	ReconnectDelay func(n uint) time.Duration

	// FirstShard is the first and lowest shard ID to connect to.
	// If it is greater than LastShard trying to access any shard will panic.
	FirstShard uint
	// LastShard is the final and highest shard ID to connect to.
	// If it is less than FirstShard trying to access any shard will panic.
	LastShard uint
	// TotalShards is the total amount of shards that the bot will indicate it will use.
	// If left unset it will be determined from the highest shard ID + 1.
	TotalShards uint

	// See gateway_events.go.
	gatewayEvents

	shardsMu      sync.RWMutex
	shards        []*Shard
	runningShards atomic.Uint64
}

var defaultGatewayURL = func() *url.URL {
	result, err := url.Parse("wss://gateway.fluxer.app")
	if err != nil {
		panic(err)
	}

	return result
}()

func (g *Gateway) initShards() {
	if g.shards != nil {
		return
	}

	if g.FirstShard > g.LastShard {
		panic(fmt.Errorf("FirstShard (%d) > LastShard(%d)", g.FirstShard, g.LastShard))
	}

	g.shards = make([]*Shard, g.LastShard-g.FirstShard+1)
	for id := uint(0); id <= g.LastShard-g.FirstShard; id++ {
		g.shards[id] = &Shard{
			gateway: g,
			id:      g.FirstShard + id,
		}
	}
}

// Shards returns the list of shards which will be connected to.
// The sharding parameters should not be changed after this is called.
func (g *Gateway) Shards() []*Shard {
	g.shardsMu.RLock()

	if g.shards == nil {
		g.shardsMu.RUnlock()
		g.shardsMu.Lock()
		defer g.shardsMu.Unlock()

		g.initShards()
	} else {
		defer g.shardsMu.RUnlock()
	}

	return g.shards
}

// Shard returns the shard by the specified ID, which may or may not be connected.
// The sharding parameters should not be changed after this is called.
func (g *Gateway) Shard(id uint) (*Shard, bool) {
	shards := g.Shards()

	if id < g.FirstShard || id > g.LastShard {
		return nil, false
	}

	return shards[id-g.FirstShard], true
}

// Shard returns the shard by the specified ID, which may or may not be connected.
// If it does not exist on this gateway object, it will panic.
// The sharding parameters should not be changed after this is called.
func (g *Gateway) ExpectShard(id uint) *Shard {
	shard, ok := g.Shard(id)
	if !ok {
		panic("shard #%d not managed by this Gateway")
	}

	return shard
}

// RunningShards returns the count of shards that are running.
func (g *Gateway) RunningShards() uint {
	return uint(g.runningShards.Load())
}

// Start attemps to starts all shards.
// The sharding parameters should not be changed after this is called.
// You may ignore this if it is not important.
func (g *Gateway) Start() error {
	var errs []error
	for _, shard := range g.Shards() {
		err := shard.Start()
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to connect shard #%d: %w",
				shard.id,
				err,
			))
		}
	}

	return errors.Join(errs...)
}

// Stop attemps to stop all running shards.
// If some of the shards were not running or already stopping an error will be returned.
func (g *Gateway) Stop() error {
	var errs []error
	for _, shard := range g.Shards() {
		err := shard.Stop()
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to stop shard #%d: %w",
				shard.id,
				err,
			))
		}
	}

	return errors.Join(errs...)
}

// Reconnect attemps to reconnect all running shards.
// If some of the shards were not running or already disconnecting/stopping an error will be returned.
func (g *Gateway) Reconnect() error {
	var errs []error
	for _, shard := range g.Shards() {
		err := shard.Reconnect()
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to reconnect shard #%d: %w",
				shard.id,
				err,
			))
		}
	}

	return errors.Join(errs...)
}

type GatewayOpcode uint

const (
	GatewayOpDispatch            GatewayOpcode = 0
	GatewayOpHeartbeat           GatewayOpcode = 1
	GatewayOpIdentify            GatewayOpcode = 2
	GatewayOpPresenceUpdate      GatewayOpcode = 3
	GatewayOpVoiceStateUpdate    GatewayOpcode = 4
	GatewayOpResume              GatewayOpcode = 6
	GatewayOpReconnect           GatewayOpcode = 7
	GatewayOpRequestGuildMembers GatewayOpcode = 8
	GatewayOpInvalidSession      GatewayOpcode = 9
	GatewayOpHello               GatewayOpcode = 10
	GatewayOpHeartbeatACK        GatewayOpcode = 11
)

type GatewayPacket struct {
	Opcode GatewayOpcode   `json:"op"`
	Data   json.RawMessage `json:"d"`
	Seq    *uint           `json:"s"`
	Event  *string         `json:"t"`
}

type Shard struct {
	// See gateway_events.go
	shardEvents

	gateway *Gateway
	id      uint

	// stateMu is the mutex for the shared state of the shard.
	stateMu       sync.Mutex
	running       bool
	latency       time.Duration
	reqDisconnect context.CancelFunc
	reqReconnect  bool
	// (end of shared state - the remaining fields are only used by a single goroutine at a time)

	conn              *websocket.Conn
	inbound           chan GatewayPacket
	outbound          chan GatewayPacket
	readErr           chan error
	writeErr          chan error
	killRead          chan struct{}
	killWrite         chan struct{}
	heartbeat         <-chan time.Time
	pendingHeartRate  time.Duration
	lastHeartbeatSent time.Time
	heartbeatACK      bool

	sessionID string
	lastSeq   uint
}

func (s *Shard) Gateway() *Gateway {
	return s.gateway
}

func (s *Shard) ID() uint {
	return s.id
}

var (
	ErrShardAlreadyRunning       = errors.New("shard already running")
	ErrShardNotRunning           = errors.New("shard not running")
	ErrShardAlreadyStopping      = errors.New("shard already stopping")
	ErrShardAlreadyDisconnecting = errors.New("shard already disconnecting")
)

// Running is true if the shard has an actively running gorountine from calling Connect.
func (s *Shard) Running() bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	return s.running
}

// Latency returns the latency if the shard is running and has determined it.
// Otherwise it returns 0, false.
func (s *Shard) Latency() (time.Duration, bool) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	return s.latency, s.latency != 0
}

// Start starts the shard on a new gorountine.
// After this, calls will return [ErrShardAlreadyRunning] until Disconnect(false) is called and the disconnection completes.
func (s *Shard) Start() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.running {
		return ErrShardAlreadyRunning
	}

	s.running = true
	s.latency = 0

	ctx, cancel := context.WithCancel(context.Background())
	s.reqDisconnect = cancel

	go s.run(ctx, cancel)
	return nil
}

// Disconnect signals for the shard to be stopped.
// If the shard is not running, this will return [ErrShardNotRunning].
// If the shard is already in the process of being stopped, this will return [ErrShardAlreadyStopping].
func (s *Shard) Stop() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if !s.running {
		return ErrShardNotRunning
	}

	if s.reqDisconnect == nil {
		if s.reqReconnect {
			s.reqReconnect = false
			return nil
		} else {
			return ErrShardAlreadyStopping
		}
	}

	s.reqDisconnect()
	s.reqDisconnect = nil
	s.reqReconnect = false
	return nil

}

// Reconnect signals for the shard to be reconnected.
// If the shard is not running, this will return [ErrShardNotRunning].
// If the shard is already in the process of being disconnected/stopped, this will return [ErrShardAlreadyDisconnecting].
func (s *Shard) Reconnect() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if !s.running {
		return ErrShardNotRunning
	}

	if s.reqDisconnect == nil {
		return ErrShardAlreadyDisconnecting
	}

	s.reqDisconnect()
	s.reqDisconnect = nil
	s.reqReconnect = true
	return nil
}

func (s *Shard) sleepTime(attempts uint) time.Duration {
	if s.gateway.ReconnectDelay != nil {
		return s.gateway.ReconnectDelay(attempts)
	} else {
		result := time.Second
		for range min(attempts, 6) - 1 {
			result *= 2
		}

		return result
	}
}

func shouldReconnectShard(code int) bool {
	switch code {
	case 4004: // authentication failed
		return false
	case 4010: // invalid shard
		return false
	case 4011: // sharding required
		return false
	case 4012: // invalid API version
		return false
	default:
		return true
	}
}

func shouldInvalidateSession(code int) bool {
	return code == 4009 // session timed out
}

func (s *Shard) resetSession() {
	s.sessionID = ""
	s.lastSeq = 0
}

func (s *Shard) run(ctx context.Context, cancel context.CancelFunc) {
	s.gateway.runningShards.Add(1)

	s.Started.emit(ShardStartedEvent{s})
	s.gateway.ShardStarted.emit(ShardStartedEvent{s})

	var attempts uint
	var sleepTime time.Duration
	var stopErr error

	defer func() {
		cancel()

		s.stateMu.Lock()
		s.running = false
		s.latency = 0
		s.stateMu.Unlock()

		s.Stopped.emit(ShardStoppedEvent{s, stopErr})
		s.gateway.ShardStopped.emit(ShardStoppedEvent{s, stopErr})

		newRunning := s.gateway.runningShards.Add(math.MaxUint64) // -1
		if newRunning == 0 {
			s.gateway.AllShardsStopped.emit(AllShardsStoppedEvent{s.gateway})
		}
	}()

	for {
		if sleepTime != 0 {
			var stop bool

			select {
			case <-ctx.Done():
				s.stateMu.Lock()
				stop = !s.reqReconnect
				if !stop {
					slog.Debug(
						"shard explicitly stopped before connect",
						slog.Any("shard", s.id),
					)
				} else {
					slog.Debug(
						"shard explicitly reconnected before connect",
						slog.Any("shard", s.id),
					)

					ctx, cancel = context.WithCancel(context.Background())
					s.reqDisconnect = cancel
				}
				s.stateMu.Unlock()
			case <-time.After(sleepTime):
			}

			if stop {
				break
			}
		}

		dialer := s.gateway.Dialer
		if dialer == nil {
			dialer = websocket.DefaultDialer
		}

		url := *defaultGatewayURL
		if s.gateway.ConnURL != nil {
			url = *s.gateway.ConnURL
		}

		query := url.Query()
		if !query.Has("v") {
			query.Add("v", "1")
		}
		url.RawQuery = query.Encode()

		slog.Debug("attempting to establish websocket connection", slog.Any("shard", s.id))

		conn, _, err := dialer.DialContext(ctx, url.String(), http.Header{})
		if errors.Is(err, context.Canceled) {
			s.stateMu.Lock()
			reconnect := s.reqReconnect
			if reconnect {
				ctx, cancel = context.WithCancel(context.Background())
				s.reqDisconnect = cancel
			}
			s.stateMu.Unlock()

			if reconnect {
				attempts = 1
				sleepTime = s.sleepTime(attempts)

				slog.Debug(
					fmt.Sprintf("shard explicitly reconnected while establishing websocket connection; reconnecting in %s", sleepTime),
					slog.Any("shard", s.id),
				)
				continue
			} else {
				slog.Debug(
					"shard explicitly stopped while establishing websocket connection",
					slog.Any("shard", s.id),
				)
				break
			}
		} else if err != nil {
			attempts++
			sleepTime = s.sleepTime(attempts)

			slog.Warn(
				fmt.Sprintf("failed to establish websocket connection; retrying in %s", sleepTime),
				slog.Any("shard", s.id),
				slog.Any("err", err),
			)
			continue
		}

		s.Connected.emit(ShardConnectedEvent{s})
		s.gateway.ShardConnected.emit(ShardConnectedEvent{s})

		connStartTime := time.Now()

		s.conn = conn
		s.inbound = make(chan GatewayPacket)
		s.outbound = make(chan GatewayPacket, 1024)
		s.readErr = make(chan error, 1)
		s.writeErr = make(chan error, 1)
		s.killRead = make(chan struct{}, 1)
		s.killWrite = make(chan struct{}, 1)
		s.heartbeat = nil
		s.heartbeatACK = true
		s.pendingHeartRate = 0

		go s.readLoop()
		go s.writeLoop()

		disconnectErr := s.controlLoop(ctx)
		var reconnect bool

		s.stateMu.Lock()
		s.latency = 0
		s.stateMu.Unlock()

		var closeErr *websocket.CloseError
		if errors.Is(disconnectErr, context.Canceled) {
			disconnectErr = nil

			s.stateMu.Lock()
			reconnect = s.reqReconnect
			if reconnect {
				ctx, cancel = context.WithCancel(context.Background())
				s.reqDisconnect = cancel
			}
			s.stateMu.Unlock()

			if reconnect {
				attempts = 1
				sleepTime = s.sleepTime(attempts)

				slog.Debug(
					fmt.Sprintf("shard explicitly disconnected; reconnecting in %s", sleepTime),
					slog.Any("shard", s.id),
					slog.Bool("reconnect", reconnect),
				)
			} else {
				slog.Debug(
					"shard explicitly disconnected",
					slog.Any("shard", s.id),
					slog.Bool("reconnect", reconnect),
				)
			}
		} else {
			if time.Since(connStartTime) < 20*time.Second {
				attempts++
			} else {
				attempts = 1
			}
			sleepTime = s.sleepTime(attempts)

			if errors.As(disconnectErr, &closeErr) {
				reconnect = shouldReconnectShard(closeErr.Code)
				if shouldInvalidateSession(closeErr.Code) {
					s.resetSession()
				}

				if reconnect {
					slog.Warn(
						fmt.Sprintf("websocket closed with %d %s; reconnecting in %s", closeErr.Code, closeErr.Text, sleepTime),
						slog.Any("shard", s.id),
					)
				} else {
					slog.Warn(
						fmt.Sprintf("websocket closed with %d %s; not reconnecting", closeErr.Code, closeErr.Text),
						slog.Any("shard", s.id),
					)
				}
			} else {
				reconnect = true
				slog.Warn(
					fmt.Sprintf("error with websocket connection; reconnecting in %s", sleepTime.String()),
					slog.Any("shard", s.id),
					slog.Any("err", disconnectErr),
				)
			}
		}

		if s.writeErr != nil {
			// TODO: maybe we could allow a custom timeout or context
			deadline := time.Now().Add(time.Second * 3)

			s.killWrite <- struct{}{}

			var err error
			// wait for write task to finish up
			select {
			case err = <-s.writeErr:
				s.writeErr = nil
			case <-time.After(time.Until(deadline)):
				slog.Warn("writing pending packet took long to send a close message", slog.Any("shard", s.id))
			}

			if err == nil && s.writeErr == nil && s.readErr != nil {
				// we haven't had a read or write error, so send a polite goodbye message
				closeCode := websocket.CloseNormalClosure
				if disconnectErr != nil {
					closeCode = websocket.CloseAbnormalClosure
				}

				slog.Debug(
					"sending close message",
					slog.Any("shard", s.id),
					slog.Int("closeCode", closeCode),
				)

				err = s.conn.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(closeCode, "laters"),
					deadline,
				)
				if err != nil {
					slog.Warn(
						"failed to send close message",
						slog.Any("shard", s.id),
						slog.Any("err", err),
					)
				}

				select {
				case err := <-s.readErr:
					s.readErr = nil

					var closed *websocket.CloseError
					if !errors.As(err, &closed) || closed.Code != closeCode {
						slog.Warn(
							"got read error before close handshake completed",
							slog.Any("shard", s.id),
							slog.Any("err", err),
						)
					}

					slog.Debug("close handshake complete", slog.Any("shard", s.id))
				case <-time.After(time.Until(deadline)):
					slog.Warn("close handshake timed out", slog.Any("shard", s.id))
				}
			}
		}

		err = s.conn.Close()
		if err != nil {
			slog.Warn(
				"error closing websocket connection",
				slog.Any("shard", s.id),
				slog.Any("err", err),
			)
		}

		if s.readErr != nil {
			s.killRead <- struct{}{}
			// wait for read task to finish up
			<-s.readErr
		}

		if s.writeErr != nil {
			// we already signalled to killWrite - we just gave up waiting
			<-s.writeErr
		}

		event := ShardDisconnectedEvent{
			Err:          disconnectErr,
			Reconnecting: reconnect,
		}
		s.Disconnected.emit(event)
		s.gateway.ShardDisconnected.emit(event)

		if !reconnect {
			stopErr = disconnectErr
			break
		}
	}
}

func (s *Shard) readLoop() {
	for {
		msgType, reader, err := s.conn.NextReader()
		if err != nil {
			s.readErr <- err
			return
		}

		if msgType != websocket.TextMessage {
			slog.Warn("don't know how to handle binary message")
			continue
		}

		var packet GatewayPacket
		err = json.NewDecoder(reader).Decode(&packet)
		if err != nil {
			s.readErr <- err
			return
		}

		select {
		case <-s.killRead:
			s.readErr <- nil
			return
		case s.inbound <- packet:
		}
	}
}

func (s *Shard) writeLoop() {
	for {
		writer, err := s.conn.NextWriter(websocket.TextMessage)
		if err != nil {
			s.writeErr <- err
			return
		}

		var packet GatewayPacket
		select {
		case <-s.killWrite:
			s.writeErr <- nil
			return
		case packet = <-s.outbound:
		}

		err = json.NewEncoder(writer).Encode(packet)
		if err != nil {
			s.writeErr <- err
			return
		}

		err = writer.Close()
		if err != nil {
			s.writeErr <- err
			return
		}
	}
}

// controlLoop handles incoming messages and the heartbeat interval.
func (s *Shard) controlLoop(ctx context.Context) error {
	for {
		select {
		case err := <-s.readErr:
			s.readErr = nil
			return err
		case err := <-s.writeErr:
			s.writeErr = nil
			return err
		case packet := <-s.inbound:
			err := s.handlePacket(packet)
			if err != nil {
				return err
			}
		case <-s.heartbeat:
			if !s.heartbeatACK {
				return fmt.Errorf("heartbeat not acknowledged (RIP)")
			}
			s.heartbeatACK = false

			if s.pendingHeartRate != 0 {
				s.heartbeat = time.Tick(s.pendingHeartRate)
				s.pendingHeartRate = 0
			}

			s.sendHeartbeat()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Shard) handlePacket(packet GatewayPacket) error {
	event := ShardPacketEvent{
		Shard:         s,
		GatewayPacket: packet,
	}

	s.PacketReceived.emit(event)
	s.gateway.ShardPacketReceived.emit(event)

	if packet.Seq != nil {
		if *packet.Seq != s.lastSeq+1 {
			slog.Warn(fmt.Sprintf("sequence number does not follow from %d: %d", s.lastSeq, *packet.Seq))
		}

		s.lastSeq = *packet.Seq
	}

	switch packet.Opcode {
	case GatewayOpHello:
		var helloData struct {
			HeartbeatInterval int64 `json:"heartbeat_interval"`
		}
		err := json.Unmarshal(packet.Data, &helloData)
		if err != nil {
			return fmt.Errorf("failed to unmarshal Hello packet data: %w", err)
		}

		if helloData.HeartbeatInterval <= 0 {
			return fmt.Errorf("heartbeat interval too short")
		}

		s.pendingHeartRate = time.Duration(helloData.HeartbeatInterval) * time.Millisecond

		// apply jitter
		initialWait, err := rand.Int(rand.Reader, big.NewInt(helloData.HeartbeatInterval+1))
		if err != nil {
			panic(fmt.Errorf("rand.Int failed: %w", err))
		}

		s.heartbeat = time.After(time.Duration(initialWait.Int64()) * time.Millisecond)

		err = s.establishSession()
		if err != nil {
			return err
		}
	case GatewayOpReconnect:
		return errors.New("receieved reconnect packet")
	case GatewayOpInvalidSession:
		var resumable bool
		err := json.Unmarshal(packet.Data, &resumable)
		if err != nil {
			return fmt.Errorf("failed to unmarshal InvalidSession packet data: %w", err)
		}

		if !resumable {
			s.resetSession()
		}

		err = s.establishSession()
		if err != nil {
			return err
		}
	case GatewayOpHeartbeat:
		s.sendHeartbeat()
	case GatewayOpHeartbeatACK:
		s.heartbeatACK = true

		latency := time.Since(s.lastHeartbeatSent)
		if latency > 0 {
			s.latency = latency
		}
	case GatewayOpDispatch:
		err := s.handleDispatch(packet)
		if err != nil {
			return fmt.Errorf("error handling Dispatch packet: %w", err)
		}
	default:
		slog.Warn("don't know how to handle " + packet.Opcode.String())
	}

	return nil
}

type gatewayIdentifyPayload struct {
	Token      string  `json:"token"`
	Shard      [2]uint `json:"shard,omitempty"`
	Properties struct {
		OS      string `json:"os"`
		Browser string `json:"browser"`
		Device  string `json:"device"`
	} `json:"properties"`
}

type gatewayResumePayload struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Seq       uint   `json:"seq"`
}

func (s *Shard) establishSession() error {
	if s.sessionID == "" {
		slog.Debug("no existing session; sending identify packet", slog.Any("shard", s.id))

		payload := gatewayIdentifyPayload{
			Token: s.gateway.Auth,
			Shard: [2]uint{s.id, s.gateway.TotalShards},
		}

		payload.Properties.OS = runtime.GOOS
		payload.Properties.Browser = defaultUserAgent
		payload.Properties.Device = defaultUserAgent

		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal Identify packet data: %w", err)
		}

		s.outbound <- GatewayPacket{
			Opcode: GatewayOpIdentify,
			Data:   data,
		}
	} else {
		slog.Debug("have existing session; sending resume packet", slog.Any("shard", s.id), slog.Any("session", s.sessionID))

		payload := gatewayResumePayload{
			Token:     s.gateway.Auth,
			SessionID: s.sessionID,
			Seq:       s.lastSeq,
		}

		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal Resume packet data: %w", err)
		}

		s.outbound <- GatewayPacket{
			Opcode: GatewayOpResume,
			Data:   data,
		}
	}

	return nil
}

type gatewayGuild struct {
	Properties Guild          `json:"properties"`
	Channels   []Channel      `json:"channels"`
	Roles      []Role         `json:"roles"`
	Members    []Member       `json:"members"`
	Emojis     []GuildEmoji   `json:"emojis"`
	Stickers   []GuildSticker `json:"stickers"`
}

func (s *Shard) handleDispatch(packet GatewayPacket) error {
	if packet.Event == nil {
		return errors.New("Dispatch packet does not contain event name")
	}

	cache := s.gateway.Cache

	switch *packet.Event {
	case "READY":
		var raw struct {
			SessionID string      `json:"session_id"`
			User      UserPrivate `json:"user"`
			Guilds    []struct {
				Unavailable bool `json:"unavailable"`
				ID          ID   `json:"ID"`
				gatewayGuild
			} `json:"guilds"`
		}
		err := json.Unmarshal(packet.Data, &raw)

		s.sessionID = raw.SessionID
		if cache != nil {
			cacheCurrentUser(&raw.User, cache)
		}

		event := ShardReadyEvent{
			Shard:     s,
			SessionID: raw.SessionID,
			User:      raw.User,
		}
		if err != nil {
			return fmt.Errorf("failed to unmarshal READY data: %w", err)
		}

		event.Guilds = make([]ReadyGuild, 0, len(raw.Guilds))
		for _, rawGuild := range raw.Guilds {
			if rawGuild.Unavailable {
				event.Guilds = append(event.Guilds, ReadyGuild{
					Unavailable: true,
					ID:          rawGuild.ID,
					Cached:      uncacheGuild(rawGuild.ID, cache),
				})
			} else {
				guild, _ := cacheGatewayGuild(&rawGuild.gatewayGuild, cache)

				event.Guilds = append(event.Guilds, ReadyGuild{
					Unavailable: false,
					ID:          rawGuild.Properties.ID,
					Guild:       &guild,
				})
			}
		}

		s.Ready.emit(event)
		s.gateway.ShardReady.emit(event)

		for _, guild := range event.Guilds {
			if guild.Unavailable {
				event := GuildRemoveEvent{
					Shard:  s,
					ID:     guild.ID,
					Cached: guild.Cached,
				}

				s.gateway.GuildUnavailable.emit(event)
			} else {
				s.gateway.GuildAvailable.emit(GuildAddEvent{
					Shard: s,
					Guild: *guild.Guild,
				})
			}
		}
	case "RESUMED":
		s.Resumed.emit(ShardResumedEvent{s})
		s.gateway.ShardResumed.emit(ShardResumedEvent{s})
	case "CHANNEL_CREATE":
		event := ChannelCreateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal CHANNEL_CREATE data: %w", err)
		}

		cacheChannel(&event.Channel, cache)
		s.gateway.ChannelCreate.emit(event)
	case "CHANNEL_UPDATE":
		event := ChannelUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal CHANNEL_UPDATE data: %w", err)
		}

		cacheChannel(&event.Channel, cache)
		s.gateway.ChannelUpdate.emit(event)
	case "CHANNEL_UPDATE_BULK":
		event := ChannelUpdateBulkEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal CHANNEL_UPDATE_BULK data: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok && guild.Channels != nil {
				for _, channel := range event.Channels {
					cacheGuildChannel(&guild, &channel, cache)
				}
			}
		}

		s.gateway.ChannelUpdateBulk.emit(event)
	case "CHANNEL_DELETE":
		event := ChannelDeleteEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal CHANNEL_DELETE data: %w", err)
		}

		uncacheChannel(&event.Channel, cache)
		s.gateway.ChannelDelete.emit(event)
	case "MESSAGE_CREATE":
		event := MessageCreateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal MESSAGE_CREATE data: %w", err)
		}

		if event.Member != nil {
			event.Member.User = event.Author
		}

		cacheGatewayMessage(&event, true, cache)
		s.gateway.MessageCreate.emit(event)
	case "MESSAGE_UPDATE":
		event := MessageUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal MESSAGE_UPDATE data: %w", err)
		}

		if event.Member != nil {
			event.Member.User = event.Author
		}

		cacheGatewayMessage((*MessageCreateEvent)(&event), false, cache)
		s.gateway.MessageUpdate.emit(event)
	case "MESSAGE_DELETE":
		event := MessageDeleteEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal MESSAGE_DELETE data: %w", err)
		}

		event.Cached = uncacheGatewayMessage(&event, cache)
		s.gateway.MessageDelete.emit(event)
	case "MESSAGE_DELETE_BULK":
		var raw struct {
			ChannelID ID   `json:"channel_id"`
			GuildID   *ID  `json:"guild_id"`
			IDs       []ID `json:"ids"`
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal MESSAGE_DELETE_BULK: %w", err)
		}

		event := MessageDeleteBulkEvent{
			ChannelID: raw.ChannelID,
			GuildID:   raw.GuildID,
			Messages:  make([]BulkDeletedMessage, 0, len(raw.IDs)),
		}

		var messages *Collection[Message]
		if cache != nil {
			if raw.GuildID != nil {
				if guild, ok := cache.Guilds.Get(*raw.GuildID); ok {
					if channel, ok := guild.Channels.Get(raw.ChannelID); ok {
						messages = channel.Messages
					}
				}
			} else {
				if channel, ok := cache.Channel(raw.ChannelID); ok {
					messages = channel.Messages
				}
			}
		}

		for _, id := range raw.IDs {
			var cached *Message
			if messages != nil {
				cached, _ = messages.Delete(id)
			}

			event.Messages = append(event.Messages, BulkDeletedMessage{
				ID: id,
				Cached: cached,
			})
		}

		s.gateway.MessageDeleteBulk.emit(event)
	case "TYPING_START":
		var raw struct {
			ChannelID ID      `json:"channel_id"`
			UserID    ID      `json:"user_id"`
			Timestamp int64   `json:"timestamp"`
			GuildID   *ID     `json:"guild_id"`
			Member    *Member `json:"member"`
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal TYPING_START data: %w", err)
		}

		if raw.GuildID != nil && raw.Member != nil {
			cacheMember(*raw.GuildID, raw.Member, cache)
		}

		event := TypingStartEvent{
			Shard:     s,
			ChannelID: raw.ChannelID,
			UserID:    raw.UserID,
			Timestamp: time.UnixMilli(raw.Timestamp),
			GuildID:   raw.GuildID,
			Member:    raw.Member,
		}
		s.gateway.TypingStart.emit(event)
	case "GUILD_CREATE":
		var raw gatewayGuild
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_CREATE data: %w", err)
		}

		guild, wasUnavailable := cacheGatewayGuild(&raw, cache)

		event := GuildAddEvent{
			Shard: s,
			Guild: guild,
		}

		if wasUnavailable {
			s.gateway.GuildAvailable.emit(event)
		} else {
			s.gateway.GuildCreate.emit(event)
		}
	case "GUILD_UPDATE":
		event := GuildUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_UPDATE data: %w", err)
		}

		cacheGuild(&event.Guild, cache)
		s.gateway.GuildUpdate.emit(event)
	case "GUILD_DELETE":
		var raw struct {
			ID          ID   `json:"id"`
			Unavailable bool `json:"unavailable"`
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_DELETE data: %w", err)
		}

		event := GuildRemoveEvent{
			Shard:  s,
			ID:     raw.ID,
			Cached: uncacheGuild(raw.ID, cache),
		}

		if raw.Unavailable {
			s.gateway.GuildUnavailable.emit(event)
		} else {
			s.gateway.GuildDelete.emit(event)
		}
	case "GUILD_ROLE_CREATE":
		var raw struct {
			GuildID ID   `json:"id"`
			Role    Role `json:"role"`
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_ROLE_CREATE data: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(raw.Role.ID)
			if ok && guild.Roles != nil {
				guild.Roles.Set(raw.Role.ID, raw.Role)
			}
		}

		event := RoleCreateEvent{
			Shard:   s,
			GuildID: raw.GuildID,
			Role:    raw.Role,
		}
		s.gateway.RoleCreate.emit(event)
	case "GUILD_ROLE_UPDATE":
		var raw struct {
			GuildID ID   `json:"guild_id"`
			Role    Role `json:"role"`
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_ROLE_UPDATE data: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(raw.GuildID)
			if ok && guild.Roles != nil {
				guild.Roles.Set(raw.Role.ID, raw.Role)
			}
		}

		event := RoleUpdateEvent{
			Shard:   s,
			GuildID: raw.GuildID,
			Role:    raw.Role,
		}
		s.gateway.RoleUpdate.emit(event)
	case "GUILD_ROLE_UPDATE_BULK":
		event := RoleUpdateBulkEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_ROLE_UPDATE_BULK data: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok && guild.Roles != nil {
				for _, role := range event.Roles {
					guild.Roles.Set(role.ID, role)
				}
			}
		}

		s.gateway.RoleUpdateBulk.emit(event)
	case "GUILD_ROLE_DELETE":
		event := RoleDeleteEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_ROLE_DELETE data: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok && guild.Roles != nil {
				role, _ := guild.Roles.Delete(event.RoleID)
				event.Cached = role
			}
		}

		s.gateway.RoleDelete.emit(event)
	case "GUILD_MEMBER_ADD":
		event := MemberAddEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_MEMBER_ADD data: %w", err)
		}

		cacheMember(event.GuildID, &event.Member, cache)
		s.gateway.MemberAdd.emit(event)
	case "GUILD_MEMBER_UPDATE":
		event := MemberUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_MEMBER_UPDATE data: %w", err)
		}

		cacheMember(event.GuildID, &event.Member, cache)
		s.gateway.MemberUpdate.emit(event)
	case "GUILD_MEMBER_REMOVE":
		var raw struct {
			GuildID ID `json:"guild_id"`
			User    struct {
				ID ID `json:"id"`
			}
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_MEMBER_REMOVE data: %w", err)
		}

		event := MemberRemoveEvent{
			Shard:    s,
			GuildID:  raw.GuildID,
			MemberID: raw.User.ID,
			Cached:   uncacheMember(raw.GuildID, raw.User.ID, cache),
		}

		s.gateway.MemberRemove.emit(event)
	case "GUILD_EMOJIS_UPDATE":
		event := GuildEmojisUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_EMOJIS_UPDATE data: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok && guild.Emojis != nil {
				// FIXME: probably should be atomic
				guild.Emojis.Clear()
				for _, emoji := range event.Emojis {
					guild.Emojis.Set(emoji.ID, emoji)
				}
			}
		}

		s.gateway.GuildEmojisUpdate.emit(event)
	case "GUILD_STICKERS_UPDATE":
		event := GuildStickersUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_STICKERS_UPDATE data: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok && guild.Stickers != nil {
				// FIXME: probably should be atomic
				guild.Stickers.Clear()
				for _, emoji := range event.Stickers {
					guild.Stickers.Set(emoji.ID, emoji)
				}
			}
		}

		s.gateway.GuildStickersUpdate.emit(event)
	case "GUILD_BAN_ADD":
		var raw struct {
			GuildID ID `json:"guild_id"`
			User    struct {
				ID ID `json:"id"`
			} `json:"user"`
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_BAN_REMOVE data: %w", err)
		}

		event := GuildBanAddEvent{
			Shard:   s,
			GuildID: raw.GuildID,
			UserID:  raw.User.ID,
		}
		s.gateway.GuildBanAdd.emit(event)
	case "GUILD_BAN_REMOVE":
		var raw struct {
			GuildID ID `json:"guild_id"`
			User    struct {
				ID ID `json:"id"`
			} `json:"user"`
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_BAN_REMOVE data: %w", err)
		}

		event := GuildBanRemoveEvent{
			Shard:   s,
			GuildID: raw.GuildID,
			UserID:  raw.User.ID,
		}
		s.gateway.GuildBanRemove.emit(event)
	case "USER_UPDATE":
		event := UserUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal USER_UPDATE data: %w", err)
		}

		cacheCurrentUser(&event.UserPrivate, cache)
		s.gateway.UserUpdate.emit(event)
	default:
		slog.Warn("don't know how to handle event " + *packet.Event)
	}

	return nil
}

func (s *Shard) sendHeartbeat() {
	s.lastHeartbeatSent = time.Now()

	data := []byte("null")
	if s.lastSeq != 0 {
		data = fmt.Append(nil, s.lastSeq)
	}

	s.outbound <- GatewayPacket{
		Opcode: GatewayOpHeartbeat,
		Data:   data,
	}
}
