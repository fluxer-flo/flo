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

// Gateway manages [Shard]s, and provides a higher level interface.
// Fluxer does not yet support sharding, but should do upon the release of v2.
// The current implementation was tested with Discord.
type Gateway struct {
	// Auth specifies the token to send when connecting.
	Auth string
	// Cache specifies the caching target. If nil is specified, nothing is cached.
	Cache *Cache
	// ConnURL specifies the initial URL for establishing a connection.
	ConnURL *url.URL
	// Dialer specifies options for connecting to the WebSocket server (from Gorilla WebSocket).
	// It nil is specified, websocket.DefaultDialer is used.
	Dialer *websocket.Dialer

	// FirstShard is the first and lowest shard ID to connect to.
	FirstShard uint
	// LastShard is the last and highest shard ID to connect to.
	LastShard uint
	// TotalShards is the total amount of shards that the bot will indicate it will use.
	// If left unset it will be determined from LastShard + 1.
	TotalShards uint

	PacketReceived Signal[GatewayPacket]

	shards          []*Shard
	currFirstShard  uint
	currLastShard   uint
	currTotalShards uint
	stateMu         sync.RWMutex
}

var ErrGatewayAlreadyConnected = errors.New("already connected")

func (g *Gateway) initShards() error {
	if g.LastShard < g.FirstShard {
		return fmt.Errorf("LastShard (%d) < FirstShard (%d)", g.LastShard, g.FirstShard)
	}

	g.shards = make([]*Shard, g.LastShard-g.FirstShard+1)
	for i := uint(0); i <= g.LastShard-g.FirstShard; i++ {
		g.shards[i] = &Shard{
			ID:    g.FirstShard + i,
			ready: make(chan struct{}, 1),
			end:   make(chan error, 1),
		}
	}

	// NOTE: this is to avoid strange behaviour with the original values being modified
	g.currFirstShard = g.FirstShard
	g.currLastShard = g.LastShard

	if g.TotalShards == 0 {
		g.currTotalShards = g.LastShard + 1
	} else {
		g.currTotalShards = g.TotalShards
	}

	return nil
}

// Connect establishes connections to the gateway and blocks until they are all finished.
func (g *Gateway) Connect() error {
	dialer := g.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}

	url := *defaultGatewayURL
	if g.ConnURL != nil {
		url = *g.ConnURL
	}

	query := url.Query()
	if !query.Has("v") {
		query.Add("v", "1")
	}
	url.RawQuery = query.Encode()

	g.stateMu.Lock()
	if g.shards != nil {
		g.stateMu.Unlock()
		return ErrGatewayAlreadyConnected
	}

	err := g.initShards()
	if err != nil {
		g.stateMu.Unlock()
		return err
	}

	for _, shard := range g.shards {
		go func() {
			shard.run(context.Background(), g, dialer, url)
		}()
	}
	g.stateMu.Unlock()

	g.stateMu.RLock()
	defer g.stateMu.RUnlock()

	for _, shard := range g.shards {
		fmt.Println("waiting on " + fmt.Sprint(shard.ID))
		select {
		case <-shard.ready:
			continue
		case err := <-shard.end:
			return fmt.Errorf("shard #%d ended before ready: %w", shard.ID, err)
		}
	}

	return nil
}

func (g *Gateway) Shard(id uint) (*Shard, bool) {
	g.stateMu.RLock()
	defer g.stateMu.RUnlock()

	if id < g.currFirstShard || id > g.currLastShard {
		return nil, false
	}

	return g.shards[id-g.FirstShard], true
}

func (g *Gateway) ShardForGuild(guildID ID) (*Shard, bool) {
	g.stateMu.RLock()
	defer g.stateMu.RUnlock()

	// NOTE: thanks to the modulo operator it will always be in range of uint
	shardID := uint(guildID >> 22 % ID(g.currTotalShards))
	return g.Shard(uint(shardID))
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
	Opcode      GatewayOpcode   `json:"op"`
	Data        json.RawMessage `json:"d"`
	SequenceNum *uint           `json:"s"`
	Event       *string         `json:"t"`
}

type Shard struct {
	ID uint

	PacketReceived Signal[GatewayPacket]

	// signals to the gateway, set only once by the gateway
	ready chan struct{}
	end   chan error

	conn     *websocket.Conn
	inbound  chan GatewayPacket
	outbound chan GatewayPacket
	readErr  chan error
	writeErr chan error

	lastSeq          *uint
	heartbeat        <-chan time.Time
	pendingHeartRate time.Duration
	heartbeatACK     bool
}

func (s *Shard) run(ctx context.Context, gateway *Gateway, dialer *websocket.Dialer, url url.URL) {
	var sleepTime time.Duration
	for {
		time.Sleep(sleepTime)
		sleepTime = time.Second

		conn, _, err := dialer.DialContext(ctx, url.String(), http.Header{})
		if err != nil {
			slog.Warn(
				"failed to establish websocket connection; retrying in "+sleepTime.String(),
				slog.Int("shard", int(s.ID)),
				slog.Any("err", err),
			)
			continue
		}

		s.conn = conn
		s.inbound = make(chan GatewayPacket)
		s.outbound = make(chan GatewayPacket, 1024)
		s.readErr = make(chan error)
		s.writeErr = make(chan error)

		go s.read()
		go s.write()

		err = s.listen(gateway)
		slog.Warn(
			"error with webhook connection; retrying in "+sleepTime.String(),
			slog.Int("shard", int(s.ID)),
		)
	}
}

func (s *Shard) read() {
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

func (s *Shard) write() {
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

func (s *Shard) listen(gateway *Gateway) error {
	for {
		select {
		case err := <-s.writeErr:
			return err
		case err := <-s.readErr:
			return err
		case packet := <-s.inbound:
			err := s.handlePacket(gateway, packet)
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

func (s *Shard) handlePacket(gateway *Gateway, packet GatewayPacket) error {
	err := errors.Join(s.PacketReceived.emit(packet), gateway.PacketReceived.emit(packet))
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
			Token: gateway.Auth,
			Shard: [2]uint{s.ID, gateway.currTotalShards},
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
		s.handleEvent(packet)
	default:
		slog.Warn("don't know how to handle " + packet.Opcode.String())
	}

	return nil
}

func (s *Shard) handleEvent(packet GatewayPacket) error {
	if packet.Event == nil {
		return errors.New("Dispatch packet does not contain event name")
	}

	event := *packet.Event
	switch event {
	case "READY":
		s.ready <- struct{}{}
		s.ready = nil
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
