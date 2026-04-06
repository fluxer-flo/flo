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

// Gateway manages Fluxer gateway connections, and provides a high level interface.
// Currently, since Fluxer does not support sharding this will only use one connection.
type Gateway struct {
	// Auth specifies the token to send when connecting.
	Auth string
	// ConnURL specifies the initial URL for establishing a connection.
	ConnURL *url.URL
	// Cache specifies the caching target. If nil is specified, nothing is cached.
	Cache *Cache
	// Dialer specifies options for connecting to the WebSocket server (from Gorilla WebSocket).
	// It nil is specified, websocket.DefaultDialer is used.
	Dialer *websocket.Dialer

	PacketReceived Signal[GatewayPacket]

	sesh   session
	seshMu sync.Mutex
}

var ErrGatewayAlreadyConnected = errors.New("already connected")

// Connect establishes a connection to the gateway and blocks until the connection has finished.
func (g *Gateway) Connect(ctx context.Context) error {
	dialer := g.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}

	url := *defaultGatewayURL
	if g.ConnURL != nil {
		url = *g.ConnURL
	}

	query := url.Query()
	query.Add("v", "1")
	url.RawQuery = query.Encode()

	g.seshMu.Lock()
	if g.sesh.active {
		g.seshMu.Unlock()
		return ErrGatewayAlreadyConnected
	}

	sesh, err := beginSession(ctx, g, dialer, url)
	g.seshMu.Unlock()
	if err != nil {
		return err
	}

	return <-sesh.finished
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

type session struct {
	active   bool
	conn     *websocket.Conn
	inbound  chan GatewayPacket
	outbound chan GatewayPacket
	readErr  chan error
	writeErr chan error
	finished chan error

	lastSeq          *uint
	heartbeat        <-chan time.Time
	pendingHeartRate time.Duration
	heartbeatACK     bool
}

func beginSession(ctx context.Context, gateway *Gateway, dialer *websocket.Dialer, url url.URL) (session, error) {
	conn, _, err := dialer.DialContext(ctx, url.String(), http.Header{})
	if err != nil {
		return session{}, fmt.Errorf("failed to create websocket connection: %w", err)
	}

	sesh := session{
		active:   true,
		conn:     conn,
		inbound:  make(chan GatewayPacket),
		outbound: make(chan GatewayPacket, 1024),
		readErr:  make(chan error),
		writeErr: make(chan error),
		finished: make(chan error),
	}

	go sesh.read()
	go sesh.write()
	go func() {
		sesh.finished <- sesh.control(gateway)
	}()

	return sesh, nil
}

func (s *session) read() {
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

func (s *session) write() {
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

func (s *session) control(gateway *Gateway) error {
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
	Token      string `json:"token"`
	Properties struct {
		OS      string `json:"os"`
		Browser string `json:"browser"`
		Device  string `json:"device"`
	} `json:"properties"`
}

func (s *session) handlePacket(gateway *Gateway, packet GatewayPacket) error {
	err := gateway.PacketReceived.emit(packet)
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
	default:
		slog.Warn("don't know how to handle " + packet.Opcode.String())
	}

	return nil
}

func (s *session) sendHeartbeat() error {
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
