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

	// PacketReceived is emitted when a packet on any of the shards is received.
	PacketReceived Signal[ShardPacketEvent]
	// GuildCreate is emitted when the user has joined a guild.
	GuildCreate Signal[GuildAddEvent]
	// GuildAvailable is emitted when a guild is no longer unavailable.
	GuildAvailable Signal[GuildAddEvent]
	// GuildDelete is emitted when a guild is deleted or the user has left it.
	GuildDelete Signal[GuildRemoveEvent]
	// GuildUnavailable is emitted when a guild is unavailable.
	GuildUnavailable Signal[GuildRemoveEvent]
	// ChannelCreate is emitted when a channel is created or opened for the user.
	ChannelCreate Signal[ChannelCreateEvent]
	// ChannelUpdate is emitted when an individual channel is updated.
	ChannelUpdate Signal[ChannelUpdateEvent]
	// ChannelUpdateBulk is emitted when multiple guild channel updates are reported at once.
	ChannelUpdateBulk Signal[ChannelUpdateBulkEvent]
	// ChannelDelete is emitted when a channel is deleted.
	ChannelDelete Signal[ChannelDeleteEvent]
	// MessageCreate is emitted when a user sends a message.
	MessageCreate Signal[MessageCreateEvent]

	shardsMu sync.RWMutex
	shards   []*Shard
}

// GuildAddEvent represents a guild becoming available or being joined.
type GuildAddEvent struct {
	Shard *Shard `json:"-"`
	Guild
}

// GuildRemoveEvent represents a guild becoming unavailable or being left/deleted.
type GuildRemoveEvent struct {
	Shard *Shard
	ID    ID
	// Cached is the guild that was removed from the cache by this event, if any.
	Cached *Guild
}

type ChannelCreateEvent struct {
	Shard *Shard `json:"-"`
	Channel
}

type ChannelUpdateEvent struct {
	Shard *Shard `json:"-"`
	Channel
}

type ChannelUpdateBulkEvent struct {
	Shard    *Shard    `json:"-"`
	GuildID  ID        `json:"guild_id"`
	Channels []Channel `json:"channels"`
}

type ChannelDeleteEvent struct {
	Shard *Shard `json:"-"`
	Channel
}

// MessageCreateEvent represents a received message.
type MessageCreateEvent struct {
	Shard   *Shard  `json:"-"`
	Member  *Member `json:"member"`
	GuildID *ID     `json:"guild_id"`
	// Nonce is a string that can be set when creating a message and checked to verify it has been sent.
	Nonce *string `json:"nonce"`
	Message
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

// Connect connects all shards which are not already connected.
// The sharding parameters should not be changed after this is called.
// If some of the shards were already connected an error will be returned.
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

// Disconnect disconnects all connected shards.
// If some of the shards were not connected an error will be returned.
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

type shardState uint

const (
	shardStateDisconnected shardState = iota
	shardStateConnected
	shardStateDisconnecting
)

type Shard struct {
	// PacketReceived is emitted when a packet is received from Fluxer.
	PacketReceived Signal[ShardPacketEvent]
	// Ready is emitted when a READY packet is received.
	// This means the login was successful and contains various information, but no guilds will yet be available on a bot account.
	Ready Signal[ShardReadyEvent]

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
	lastSeq          uint
	heartbeat        <-chan time.Time
	pendingHeartRate time.Duration
	heartbeatACK     bool
}

type ShardPacketEvent struct {
	Shard *Shard `json:"-"`
	GatewayPacket
}

type ShardReadyEvent struct {
	Shard *Shard      `json:"-"`
	User  UserPrivate `json:"user"`
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
		s.lastSeq = 0
		s.heartbeat = nil
		s.pendingHeartRate = 0
		s.heartbeatACK = true

		go s.readLoop()
		go s.writeLoop()

		err = s.controlLoop()
		reconnect := true

		var closeErr *websocket.CloseError
		if errors.As(err, &closeErr) {
			reconnect = shouldReconnectShard(closeErr.Code)
		}

		if reconnect {
			slog.Warn(
				"error with webhook connection; reconnecting in "+sleepTime.String(),
				slog.Any("shard", s.ID),
				slog.Any("err", err),
			)
		} else {
			slog.Warn(
				"unrecoverable error with webhook connection; not reconnecting",
				slog.Any("shard", s.ID),
				slog.Any("err", err),
			)
		}

		err = s.conn.Close()
		if err != nil {
			slog.Warn(
				"error closing webhook connection",
				slog.Any("shard", s.ID),
				slog.Any("err", err),
			)
		}

		if !reconnect {
			break
		}
	}

	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.state = shardStateDisconnected
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
			if !s.heartbeatACK {
				return fmt.Errorf("heartbeat not acknowledged (RIP)")
			}

			if s.pendingHeartRate != 0 {
				s.heartbeat = time.Tick(s.pendingHeartRate)
				s.pendingHeartRate = 0
			}

			s.sendHeartbeat()
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
		if *packet.SequenceNum != s.lastSeq+1 {
			slog.Warn(fmt.Sprintf("sequence number does not follow from %d: %d", s.lastSeq, *packet.SequenceNum))
		}

		s.lastSeq = *packet.SequenceNum
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
			Shard: [2]uint{s.id, s.gateway.TotalShards},
		}

		payload.Properties.OS = runtime.GOOS
		payload.Properties.Browser = defaultUserAgent
		payload.Properties.Device = defaultUserAgent

		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal identify packet data: %w", err)
		}

		s.outbound <- GatewayPacket{
			Opcode: GatewayOpIdentify,
			Data:   data,
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

func (s *Shard) handleDispatch(packet GatewayPacket) error {
	if packet.Event == nil {
		return errors.New("Dispatch packet does not contain event name")
	}

	cache := s.gateway.Cache

	switch *packet.Event {
	case "READY":
		event := ShardReadyEvent{Shard: s}
		err := json.Unmarshal(packet.Data, &event)
		if err != nil {
			return fmt.Errorf("failed to unmarshal READY data: %w", err)
		}

		s.Ready.emit(event)
	case "GUILD_CREATE":
		var raw struct {
			Properties Guild     `json:"properties"`
			Channels   []Channel `json:"channels"`
			Roles      []Role    `json:"roles"`
		}
		err := json.Unmarshal(packet.Data, &raw)
		if err != nil {
			return fmt.Errorf("failed to unmarshal GUILD_CREATE data: %w", err)
		}

		event := GuildAddEvent{
			Shard: s,
			Guild: newGuildForCache(raw.Properties.ID, cache),
		}
		event.Guild.updateProperties(&raw.Properties)

		event.Guild.Channels.Clear()
		for _, channel := range raw.Channels {
			event.Guild.Channels.Set(channel.ID, channel)
		}

		event.Guild.Roles.Clear()
		for _, role := range raw.Roles {
			event.Guild.Roles.Set(role.ID, role)
		}

		if cache != nil {
			cache.Guilds.Set(event.Guild.ID, event.Guild)

			if _, ok := cache.UnavailableGuilds.Delete(raw.Properties.ID); ok {
				s.gateway.GuildAvailable.emit(event)
				break
			}
		}

		s.gateway.GuildCreate.emit(event)
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
			if guild, ok := cache.Guilds.Delete(raw.ID); ok {
				event.Cached = guild
			}

			if raw.Unavailable {
				s.gateway.Cache.UnavailableGuilds.Set(raw.ID, struct{}{})
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

			if event.GuildID != nil {
				guild, ok := cache.Guilds.Get(*event.GuildID)
				if ok && guild.Channels != nil {
					guild.Channels.Update(event.ChannelID, func(val *Channel) {
						id := event.Message.ID
						val.LastMessageID = &id
					})
				}
			}
		}

		s.gateway.MessageCreate.emit(event)
	default:
		slog.Warn("don't know how to handle event " + *packet.Event)

	}

	return nil
}

func (s *Shard) marshalSeq() []byte {
	if s.lastSeq == 0 {
		return []byte("null")
	} else {
		return fmt.Append(nil, s.lastSeq)
	}

}

func (s *Shard) sendHeartbeat() {
	s.heartbeatACK = false
	s.outbound <- GatewayPacket{
		Opcode: GatewayOpHeartbeat,
		Data:   s.marshalSeq(),
	}
}
