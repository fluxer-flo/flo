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

	// FirstShard is the first and lowest shard ID to connect to.
	// If it is greater than LastShard trying to access any shard will panic.
	FirstShard uint
	// LastShard is the final and highest shard ID to connect to.
	// If it is less than FirstShard trying to access any shard will panic.
	LastShard uint
	// TotalShards is the total amount of shards that the bot will indicate it will use.
	// If left unset it will be determined from the highest shard ID + 1.
	TotalShards uint

	PacketReceived Signal[ShardPacketEvent]

	shardsMu sync.RWMutex
	shards   []*Shard
	// backed up fields in case they change
	firstShard  uint
	lastShard   uint
	totalShards uint
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
			id:      id,
		}
	}

	g.firstShard = g.FirstShard
	g.lastShard = g.LastShard

	if g.TotalShards == 0 {
		g.totalShards = g.lastShard + 1
	} else {
		g.totalShards = g.TotalShards
	}
}

// Shard returns the shard by the specified ID, which may or may not be connected.
// This will cause the internal list of [Shard]s to be populated (if not already) after which any changes to FirstShard, LastShard or TotalShards will be ignored.
func (g *Gateway) Shard(id uint) (*Shard, bool) {
	g.shardsMu.RLock()

	if g.shards == nil {
		g.shardsMu.RUnlock()
		g.shardsMu.Lock()
		defer g.shardsMu.Unlock()

		g.initShards()
	} else {
		defer g.shardsMu.RUnlock()
	}

	if id < g.firstShard || id > g.lastShard {
		return nil, false
	}

	return g.shards[id-g.firstShard], true
}

// Connect connects all shards which are not already connected.
// This will cause the internal list of [Shard]s to be populated (if not already) after which any changes to FirstShard, LastShard or TotalShards will be ignored.
// If some of the shards were already connected an error will be returned.
// You may ignore this if it is not important.
func (g *Gateway) Connect() error {
	g.shardsMu.Lock()
	if g.shards == nil {
		g.initShards()
	}
	g.shardsMu.Unlock()

	g.shardsMu.RLock()
	defer g.shardsMu.RUnlock()

	var errs []error
	for _, shard := range g.shards {
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

// Disconnect disconnects all connected shards.
// If some of the shards were not connected an error will be returned.
func (g *Gateway) Disconnect(reconnect bool) error {
	g.shardsMu.RLock()
	defer g.shardsMu.RUnlock()

	var errs []error
	for _, shard := range g.shards {
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
type shardState uint

const (
	shardStateDisconnected shardState = iota
	shardStateConnected
	shardStateDisconnecting
)

type Shard struct {
	PacketReceived Signal[ShardPacketEvent]

	gateway *Gateway
	id      uint

	stateMu    sync.Mutex
	state      shardState
	disconnect chan bool // bool = whether to reconnect

	conn             *websocket.Conn
	inbound          chan GatewayPacket
	outbound         chan GatewayPacket
	readErr          chan error
	writeErr         chan error
	lastSeq          *uint
	heartbeat        <-chan time.Time
	pendingHeartRate time.Duration
	heartbeatACK     bool
}

type ShardPacketEvent struct {
	Shard *Shard
	GatewayPacket
}

type GatewayPacket struct {
	Opcode      GatewayOpcode   `json:"op"`
	Data        json.RawMessage `json:"d"`
	SequenceNum *uint           `json:"s"`
	Event       *string         `json:"t"`
}

func (s *Shard) Gateway() *Gateway {
	return s.gateway
}

func (s *Shard) ID() uint {
	return s.id
}

var (
	ErrShardAlreadyConnected     = errors.New("shard already connected")
	ErrShardAlreadyDisconnecting = errors.New("shard already disconnecting")
	ErrShardAlreadyDisconnected  = errors.New("shard already disconnected")
)

func (s *Shard) Connect() error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	if s.state != shardStateDisconnected {
		return ErrShardAlreadyConnected
	}

	s.state = shardStateConnected
	go s.run()
	return nil
}

func (s *Shard) Disconnect(reconnect bool) error {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()

	switch s.state {
	case shardStateDisconnected:
		return ErrShardAlreadyDisconnected
	case shardStateDisconnecting:
		return ErrShardAlreadyDisconnecting
	}

	s.state = shardStateDisconnecting
	s.disconnect <- reconnect
	return nil
}

func (s *Shard) run() {
	var sleepTime time.Duration
	for {
		time.Sleep(sleepTime)
		sleepTime = time.Second

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

		conn, _, err := dialer.DialContext(context.TODO(), url.String(), http.Header{})
		if err != nil {
			slog.Warn(
				"failed to establish websocket connection; retrying in "+sleepTime.String(),
				slog.Any("shard", s.id),
				slog.Any("err", err),
			)
			continue
		}

		s.conn = conn
		s.inbound = make(chan GatewayPacket)
		s.outbound = make(chan GatewayPacket, 1024)
		s.readErr = make(chan error)
		s.writeErr = make(chan error)

		go s.readLoop()
		go s.writeLoop()

		err = s.controlLoop()
		slog.Warn(
			"error with webhook connection; retrying in "+sleepTime.String(),
			slog.Any("shard", s.ID),
			slog.Any("err", err),
		)

		err = s.conn.Close()
		if err != nil {
			slog.Warn(
				"error closing webhook connection",
				slog.Any("shard", s.ID),
				slog.Any("err", err),
			)
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
func (s *Shard) controlLoop() error {
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
			err := s.sendHeartbeat()
			if err != nil {
				return err
			}
		}
	}
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

func (s *Shard) handlePacket(packet GatewayPacket) error {
	event := ShardPacketEvent{
		Shard:         s,
		GatewayPacket: packet,
	}

	err := errors.Join(s.PacketReceived.emit(event), s.gateway.PacketReceived.emit(event))
	if err != nil {
		slog.Warn("error in PacketReceived handler", slog.Any("err", err))
	}

	if packet.SequenceNum != nil {
		var expectedSeq uint = 1
		if s.lastSeq != nil {
			expectedSeq = *s.lastSeq + 1
		}

		if *packet.SequenceNum != expectedSeq {
			if s.lastSeq != nil {
				slog.Warn(fmt.Sprintf("sequence number does not follow from %d: %d", *s.lastSeq, *packet.SequenceNum))
			} else {
				slog.Warn(fmt.Sprintf("initial sequence number is not %d: %d", expectedSeq, *packet.SequenceNum))
			}
		}

		s.lastSeq = packet.SequenceNum
	}

	switch packet.Opcode {
	case GatewayOpHello:
		var helloData struct {
			HeartbeatInterval int64 `json:"heartbeat_interval"`
		}
		err := json.Unmarshal(packet.Data, &helloData)
		if err != nil {
			return fmt.Errorf("failed to decode hello packet data: %w", err)
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

		payload := gatewayIdentifyPayload{
			Token: s.gateway.Auth,
			Shard: [2]uint{s.id, s.gateway.totalShards},
		}

		payload.Properties.OS = runtime.GOOS
		payload.Properties.Browser = defaultUserAgent
		payload.Properties.Device = defaultUserAgent

		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal identify packet data: %w", err)
		}

		fmt.Println(string(data))

		s.outbound <- GatewayPacket{
			Opcode: GatewayOpIdentify,
			Data:   data,
		}
	case GatewayOpHeartbeat:
		if !s.heartbeatACK {
			return fmt.Errorf("heartbeat not acknowledged (RIP)")
		}

		if s.pendingHeartRate != 0 {
			s.pendingHeartRate = 0
			s.heartbeat = time.Tick(s.pendingHeartRate)
		}

		err := s.sendHeartbeat()
		if err != nil {
			return err
		}
	case GatewayOpHeartbeatACK:
		s.heartbeatACK = true
	case GatewayOpDispatch:
		s.handleDispatch(packet)
	default:
		slog.Warn("don't know how to handle " + packet.Opcode.String())
	}

	return nil
}

func (s *Shard) handleDispatch(packet GatewayPacket) error {
	if packet.Event == nil {
		return errors.New("Dispatch packet does not contain event name")
	}

	return nil
}

func (s *Shard) sendHeartbeat() error {
	s.heartbeatACK = false

	data, err := json.Marshal(s.lastSeq)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat data: %w", err)
	}

	s.outbound <- GatewayPacket{
		Opcode: GatewayOpHeartbeat,
		Data:   data,
	}
	return nil
}
