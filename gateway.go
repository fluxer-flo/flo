package flo

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"runtime"
	"sync"
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

	shardsMu sync.RWMutex
	shards   []*Shard
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

// Connect attemps to starts all shards.
// The sharding parameters should not be changed after this is called.
// If some of the shards were already running an error will be returned.
// You may ignore this if it is not important.
func (g *Gateway) Connect() error {
	var errs []error
	for _, shard := range g.Shards() {
		err := shard.Connect()
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

// Disconnect attemps to disconnect all running shards.
// If some of the shards were not running or already disconnecting an error will be returned.
func (g *Gateway) Disconnect(reconnect bool) error {
	var errs []error
	for _, shard := range g.Shards() {
		err := shard.Disconnect(reconnect)
		if err != nil {
			errs = append(errs, fmt.Errorf(
				"failed to disconnect shard #%d: %w",
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
	log     *slog.Logger

	// stateMu is the mutex for the shared state of the shard.
	stateMu       sync.Mutex
	running       bool
	reqDisconnect context.CancelFunc
	reqReconnect  bool
	// (end of shared state - the remaining fields are only used by a single goroutine at a time)

	conn             *websocket.Conn
	inbound          chan GatewayPacket
	outbound         chan GatewayPacket
	readErr          chan error
	writeErr         chan error
	heartbeat        <-chan time.Time
	pendingHeartRate time.Duration
	heartbeatACK     bool

	sessionID string
	lastSeq   uint
}

type shardState uint

const (
	shardStateDisconnected shardState = iota
	shardStateDisconnecting
	shardStateConnecting
	shardStateConnected
)

func (s *Shard) Gateway() *Gateway {
	return s.gateway
}

func (s *Shard) ID() uint {
	return s.id
}

var (
	ErrShardAlreadyRunning       = errors.New("shard already running")
	ErrShardNotRunning           = errors.New("shard not running")
	ErrShardAlreadyDisconnecting = errors.New("shard already disconnecting")
)

// Connect starts the shard on a new gorountine.
// After this, calls will return [ErrShardAlreadyRunning] until Disconnect(false) is called and the disconnection completes.
func (s *Shard) Connect() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.running {
		return ErrShardAlreadyRunning
	}

	s.running = true

	ctx, cancel := context.WithCancel(context.Background())
	s.reqDisconnect = cancel

	go s.run(ctx, cancel)
	return nil
}

// Disconnect signals for the shard to be disconnected.
// If reconnect is true, it will try to reconnect again after disconnnecting.
// If there is already a pending disconnect, this will return [ErrShardAlreadyDisconnecting].
func (s *Shard) Disconnect(reconnect bool) error {
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
	s.reqReconnect = reconnect
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
	var attempts uint
	var sleepTime time.Duration

	for {
		if attempts != 0 {
			ctx, cancel = context.WithCancel(context.Background())

			s.stateMu.Lock()
			s.reqDisconnect = cancel
			s.stateMu.Unlock()
		}

		if sleepTime != 0 {
			var disconnect bool

			select {
			case <-ctx.Done():
				slog.Debug(
					"shard explicitly disconnected before connect",
					slog.Any("shard", s.ID),
					slog.Bool("reconnect", s.reqReconnect),
				)

				s.stateMu.Lock()
				disconnect = !s.reqReconnect
				s.stateMu.Unlock()
			case <-time.After(sleepTime):
			}

			if disconnect {
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
			s.stateMu.Unlock()

			if reconnect {
				attempts = 1
				sleepTime = s.sleepTime(attempts)

				slog.Debug(
					fmt.Sprintf("shard explicitly disconnected while establishing websocket connection; reconnecting in %s", sleepTime),
					slog.Any("shard", s.ID),
					slog.Bool("reconnect", reconnect),
				)
				continue
			} else {
				slog.Debug(
					"shard explicitly disconnected while establishing websocket connection",
					slog.Any("shard", s.ID),
					slog.Bool("reconnect", reconnect),
				)
				break
			}
		} else if err != nil {
			attempts++
			sleepTime = s.sleepTime(attempts)

			cancel()
			slog.Warn(
				fmt.Sprintf("failed to establish websocket connection; retrying in %s", sleepTime),
				slog.Any("shard", s.id),
				slog.Any("err", err),
			)
			continue
		}

		connStartTime := time.Now()

		s.conn = conn
		s.inbound = make(chan GatewayPacket)
		s.outbound = make(chan GatewayPacket, 1024)
		s.readErr = make(chan error)
		s.writeErr = make(chan error)
		s.heartbeat = nil
		s.heartbeatACK = true
		s.pendingHeartRate = 0

		go s.readLoop()
		go s.writeLoop()

		err = s.controlLoop(ctx)
		var reconnect bool

		var closeErr *websocket.CloseError
		if errors.Is(err, context.Canceled) {
			s.stateMu.Lock()
			reconnect = s.reqReconnect
			s.stateMu.Unlock()

			if reconnect {
				attempts = 1
				sleepTime = s.sleepTime(attempts)

				slog.Debug(
					fmt.Sprintf("shard explicitly disconnected; reconnecting in %s", sleepTime),
					slog.Any("shard", s.ID),
					slog.Bool("reconnect", reconnect),
				)
			} else {
				slog.Debug(
					"shard explicitly disconnected",
					slog.Any("shard", s.ID),
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

			if errors.As(err, &closeErr) {
				reconnect = shouldReconnectShard(closeErr.Code)
				if shouldInvalidateSession(closeErr.Code) {
					s.resetSession()
				}

				if reconnect {
					slog.Warn(
						fmt.Sprintf("websocket closed with %d %s; reconnecting in %s", closeErr.Code, closeErr.Text, sleepTime),
						slog.Any("shard", s.ID),
					)
				} else {
					slog.Warn(
						fmt.Sprintf("websocket closed with %d %s; not reconnecting", closeErr.Code, closeErr.Text),
						slog.Any("shard", s.ID),
					)
				}
			} else {
				reconnect = true
				slog.Warn(
					fmt.Sprintf("error with webhook connection; reconnecting in %s", sleepTime.String()),
					slog.Any("shard", s.ID),
					slog.Any("err", err),
				)
			}
		}

		event := ShardDisconnectEvent{
			Err:          err,
			Reconnecting: reconnect,
		}
		s.Disconnected.emit(event)
		s.gateway.ShardDisconnected.emit(event)

		err = s.conn.Close()
		if err != nil {
			slog.Warn(
				"error closing webhook connection",
				slog.Any("shard", s.ID),
				slog.Any("err", err),
			)
		}

		cancel()

		if !reconnect {
			break
		}
	}

	s.stateMu.Lock()
	s.running = false
	s.stateMu.Unlock()
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
			slog.Error("failed to decode packet", slog.Any("err", err))
			continue
		}

		s.inbound <- packet
	}
}

func (s *Shard) writeLoop() {
	for {
		packet := <-s.outbound
		writer, err := s.conn.NextWriter(websocket.TextMessage)
		if err != nil {
			s.writeErr <- err
			return
		}

		err = json.NewEncoder(writer).Encode(packet)
		if err != nil {
			slog.Error("failed to encode packet", slog.Any("err", err))
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
		case err := <-s.writeErr:
			return err
		case err := <-s.readErr:
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
			cache.UpdateCurrentUser(raw.User)
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
				if cache != nil {
					cache.UnavailableGuilds.Set(rawGuild.ID, struct{}{})
				}

				event.Guilds = append(event.Guilds, ReadyGuild{
					Unavailable: true,
					ID:          rawGuild.ID,
				})
			} else {
				guild := newGuildForCache(cache)
				guild.updateGateway(&rawGuild.gatewayGuild, cache)

				if cache != nil {
					cache.Guilds.Set(guild.ID, guild)
					cache.UnavailableGuilds.Delete(guild.ID)
				}

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
					Shard: s,
					ID:    guild.ID,
				}

				if cache != nil {
					if cached, ok := cache.Guilds.Delete(guild.ID); ok {
						event.Cached = cached
					}
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
		s.Resumed.emit(ShardResumeEvent{s})
		s.gateway.ShardResumed.emit(ShardResumeEvent{s})
	case "GUILD_CREATE":
		var raw gatewayGuild
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_CREATE data: %w", err)
		}

		event := GuildAddEvent{
			Shard: s,
			Guild: newGuildForCache(cache),
		}
		event.Guild.updateGateway(&raw, cache)

		if cache != nil {
			cache.Guilds.Set(event.Guild.ID, event.Guild)

			if _, ok := cache.UnavailableGuilds.Delete(raw.Properties.ID); ok {
				s.gateway.GuildAvailable.emit(event)
				break
			}
		}

		s.gateway.GuildCreate.emit(event)
	case "GUILD_UPDATE":
		event := GuildUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_UPDATE data: %w", err)
		}

		if cache != nil {
			cache.Guilds.Update(event.Guild.ID, func(cached *Guild) {
				cached.updateProperties(&event.Guild)
				event.Guild = *cached
			})
		}

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
			Shard: s,
			ID:    raw.ID,
		}

		if cache != nil {
			if cached, ok := cache.Guilds.Delete(raw.ID); ok {
				event.Cached = cached
			}

			if raw.Unavailable {
				cache.UnavailableGuilds.Set(raw.ID, struct{}{})
			}
		}

		if raw.Unavailable {
			s.gateway.GuildUnavailable.emit(event)
		} else {
			s.gateway.GuildDelete.emit(event)
		}
	case "CHANNEL_CREATE":
		event := ChannelCreateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal CHANNEL_CREATE data: %w", err)
		}

		if cache != nil {
			if event.Channel.GuildID != nil {
				guild, ok := cache.Guilds.Get(*event.Channel.GuildID)
				if ok && guild.Channels != nil {
					guild.Channels.Set(event.ID, event.Channel)
				}
			}
		}

		s.gateway.ChannelCreate.emit(event)
	case "CHANNEL_UPDATE":
		event := ChannelUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal CHANNEL_UPDATE data: %w", err)
		}

		if cache != nil {
			if event.Channel.GuildID != nil {
				guild, ok := cache.Guilds.Get(*event.Channel.GuildID)
				if ok && guild.Channels != nil {
					guild.Channels.Set(event.ID, event.Channel)
				}
			}
		}

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
					guild.Channels.Set(channel.ID, channel)
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

		if cache != nil {
			if event.Channel.GuildID != nil {
				guild, ok := cache.Guilds.Get(*event.Channel.GuildID)
				if ok && guild.Channels != nil {
					guild.Channels.Delete(event.Channel.ID)
				}
			}
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
			guild, ok := cache.Guilds.Get(raw.GuildID)
			if ok {
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
			GuildID ID   `json:"id"`
			Role    Role `json:"role"`
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_ROLE_UPDATE data: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(raw.GuildID)
			if ok {
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
			return fmt.Errorf("failed to unmarshal GUILD_ROLE_DELETE: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok && guild.Roles != nil {
				if removed, ok := guild.Roles.Delete(event.RoleID); ok {
					event.Cached = removed
				}
			}
		}

		s.gateway.RoleDelete.emit(event)
	case "GUILD_MEMBER_ADD":
		event := MemberAddEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_MEMBER_ADD: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok && guild.Members != nil {
				guild.Members.Set(event.Member.ID(), event.Member)
			}
		}

		s.gateway.MemberAdd.emit(event)
	case "GUILD_MEMBER_UPDATE":
		event := MemberUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_MEMBER_UPDATE: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok && guild.Members != nil {
				guild.Members.Set(event.Member.ID(), event.Member)
			}
		}

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
			return fmt.Errorf("failed to unmarshal GUILD_MEMBER_REMOVE: %w", err)
		}

		event := MemberRemoveEvent{
			Shard:    s,
			GuildID:  raw.GuildID,
			MemberID: raw.User.ID,
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(raw.GuildID)
			if ok && guild.Members != nil {
				if removed, ok := guild.Members.Delete(event.MemberID); ok {
					event.Cached = removed
				}
			}
		}

		s.gateway.MemberRemove.emit(event)
	case "GUILD_EMOJIS_UPDATE":
		event := GuildEmojisUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_EMOJIS_UPDATE: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok {
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
			return fmt.Errorf("failed to unmarshal GUILD_STICKERS_UPDATE: %w", err)
		}

		if cache != nil {
			guild, ok := cache.Guilds.Get(event.GuildID)
			if ok {
				// FIXME: probably should be atomic
				guild.Stickers.Clear()
				for _, emoji := range event.Stickers {
					guild.Stickers.Set(emoji.ID, emoji)
				}
			}
		}

		s.gateway.GuildStickersUpdate.emit(event)
	case "MESSAGE_CREATE":
		event := MessageCreateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal MESSAGE_CREATE data: %w", err)
		}

		if event.Member != nil {
			event.Member.User = event.Message.Author
		}

		if cache != nil {
			event.Message.updateCache(cache)

			updateChannel := func(cached *Channel) {
				lastMessageID := event.Message.ID
				cached.LastMessageID = &lastMessageID

				if cached.Messages != nil {
					cached.Messages.Set(event.Message.ID, event.Message)
				}
			}

			if event.GuildID != nil {
				guild, ok := cache.Guilds.Get(*event.GuildID)
				if ok && guild.Channels != nil {
					guild.Channels.Update(event.ChannelID, updateChannel)
				}
				if ok && event.Member != nil && guild.Members != nil {
					guild.Members.Set(event.Member.ID(), *event.Member)
				}
			}
		}

		s.gateway.MessageCreate.emit(event)
	case "MESSAGE_UPDATE":
		event := MessageUpdateEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal MESSAGE_UPDATE data")
		}

		// FIXME: horrendous levels of indentation
		if cache != nil {
			event.Message.updateCache(cache)

			if event.GuildID != nil {
				guild, ok := cache.Guilds.Get(*event.GuildID)
				if ok {
					if guild.Channels != nil {
						if channel, ok := guild.Channels.Get(event.ChannelID); ok {
							if channel.Messages != nil {
								channel.Messages.Set(event.Message.ID, event.Message)
							}
						}
					}
					if event.Member != nil && guild.Members != nil {
						guild.Members.Set(event.Member.ID(), *event.Member)
					}
				}
			}
		}

		s.gateway.MessageUpdate.emit(event)
	case "MESSAGE_DELETE":
		event := MessageDeleteEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal MESSAGE_DELETE payload: %w", err)
		}

		// FIXME: horrendous levels of indentation
		if cache != nil && event.GuildID != nil {
			guild, ok := cache.Guilds.Get(*event.GuildID)
			if ok {
				if guild.Channels != nil {
					if channel, ok := guild.Channels.Get(event.ChannelID); ok {
						if channel.Messages != nil {
							if cached, ok := channel.Messages.Delete(event.MessageID); ok {
								event.Cached = cached
							}
						}
					}
				}
				if event.Member != nil && guild.Members != nil {
					guild.Members.Set(event.Member.ID(), *event.Member)
				}
			}
		}

		s.gateway.MessageDelete.emit(event)
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

		raw.Member.updateCache(cache)

		event := TypingStartEvent{
			Shard:     s,
			ChannelID: raw.ChannelID,
			UserID:    raw.UserID,
			Timestamp: time.UnixMilli(raw.Timestamp),
			GuildID:   raw.GuildID,
			Member:    raw.Member,
		}
		s.gateway.TypingStart.emit(event)
	default:
		slog.Warn("don't know how to handle event " + *packet.Event)
	}

	return nil
}

func (s *Shard) sendHeartbeat() {
	s.heartbeatACK = false

	data := []byte("null")
	if s.lastSeq != 0 {
		data = fmt.Append(nil, s.lastSeq)
	}

	s.outbound <- GatewayPacket{
		Opcode: GatewayOpHeartbeat,
		Data:   data,
	}
}
