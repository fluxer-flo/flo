package flo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

	connected bool
	outbound  chan<- GatewayPacket
	stateMu   sync.Mutex
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

	finished, err := g.beginSession(ctx, dialer, url)
	if err != nil {
		return err
	}

	return <-finished
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

func (g *Gateway) beginSession(ctx context.Context, dialer *websocket.Dialer, url url.URL) (<-chan error, error) {
	g.stateMu.Lock()
	defer func() {
		g.connected = true
		g.stateMu.Unlock()
	}()

	if g.connected {
		return nil, ErrGatewayAlreadyConnected
	}

	ws, _, err := dialer.DialContext(ctx, url.String(), http.Header{})
	if err != nil {
		return nil, fmt.Errorf("failed to create websocket connection: %w", err)
	}

	inbound := make(chan GatewayPacket)
	outbound := make(chan GatewayPacket, 1024)
	g.outbound = outbound

	readErr := make(chan error, 1)
	writeErr := make(chan error, 1)

	finished := make(chan error, 1)

	go g.sessionReadLoop(ws, inbound, readErr)
	go g.sessionWriteLoop(ws, outbound, writeErr)
	go func() {
		finished <- g.sessionControlLoop(inbound, readErr, writeErr)
	}()

	return finished, nil
}

func (g *Gateway) sessionReadLoop(ws *websocket.Conn, packets chan<- GatewayPacket, readErr chan<- error) {
	for {
		msgType, reader, err := ws.NextReader()
		if err != nil {
			readErr <- err
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

		packets <- packet
	}
}

func (g *Gateway) sessionWriteLoop(ws *websocket.Conn, packets <-chan GatewayPacket, writeErr chan<- error) {
	for {
		packet := <-packets
		writer, err := ws.NextWriter(websocket.TextMessage)
		if err != nil {
			writeErr <- err
			return
		}

		err = json.NewEncoder(writer).Encode(packet)
		if err != nil {
			slog.Error("failed to encode packet", slog.Any("err", err))
		}

		err = writer.Close()
		if err != nil {
			writeErr <- err
			return
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

func (g *Gateway) sessionControlLoop(inbound <-chan GatewayPacket, readErr <-chan error, writeErr <-chan error) error {
	var heartbeat <-chan time.Time
	var lastSeq *uint

	sendHeartbeat := func() error {
		data, err := json.Marshal(lastSeq)
		if err != nil {
			return fmt.Errorf("failed to marshal heartbeat data: %w", err)
		}

		g.outbound <- GatewayPacket{
			Opcode: GatewayOpHeartbeat,
			Data:   data,
		}
		return nil
	}

	for {
		select {
		case err := <-writeErr:
			return err
		case err := <-readErr:
			return err
		case packet := <-inbound:
			err := g.PacketReceived.emit(packet)
			if err != nil {
				slog.Warn("error in PacketReceived handler", slog.Any("err", err))
			}


			if packet.Opcode == GatewayOpHello {
				var helloData struct {
					HeartbeatInterval int64 `json:"heartbeat_interval"`
				}
				err := json.Unmarshal(packet.Data, &helloData)
				if err != nil {
					return fmt.Errorf("failed to decode hello packet data: %w", err)
				}

				heartbeat = time.Tick(time.Millisecond * time.Duration(helloData.HeartbeatInterval))

				payload := gatewayIdentifyPayload{
					Token: g.Auth,
				}
				payload.Properties.OS = runtime.GOOS
				payload.Properties.Browser = defaultUserAgent
				payload.Properties.Device = defaultUserAgent

				data, err := json.Marshal(payload)
				if err != nil {
					return fmt.Errorf("failed to marshal identify packet data: %w", err)
				}

				g.outbound <- GatewayPacket{
					Opcode: GatewayOpIdentify,
					Data:   data,
				}
			} else if packet.Opcode == GatewayOpHeartbeat {
				err := sendHeartbeat()
				if err != nil {
					return err
				}
			}
		case <-heartbeat:
			err := sendHeartbeat()
			if err != nil {
				return err
			}
		}
	}

}
