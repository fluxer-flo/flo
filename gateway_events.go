package flo

import "time"

type gatewayEvents struct {
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
	// MessageUpdate is emitted when a message is updated (not necessarily a user edit).
	MessageUpdate Signal[MessageUpdateEvent]
	// MessageDelete is emitted when a message is deleted.
	MessageDelete Signal[MessageDeleteEvent]
	// MessageDeleteBulk is emitted when messages are bulk-deleted.
	MessageDeleteBulk Signal[MessageDeleteBulkEvent]
	// TypingStart is emitted when a user starts typing in a channel.
	TypingStart Signal[TypingStartEvent]
	// GuildCreate is emitted when the user has joined a guild.
	GuildCreate Signal[GuildAddEvent]
	// GuildAvailable is emitted when a guild is no longer unavailable.
	GuildAvailable Signal[GuildAddEvent]
	// GuildUpdate is emitted when a guild is updated.
	GuildUpdate Signal[GuildUpdateEvent]
	// GuildDelete is emitted when a guild is deleted or the user has left it.
	GuildDelete Signal[GuildRemoveEvent]
	// GuildUnavailable is emitted when a guild is unavailable.
	GuildUnavailable Signal[GuildRemoveEvent]
	// RoleCreate is emitted when a guild role is created.
	RoleCreate Signal[RoleCreateEvent]
	// RoleUpdate is emitted when a guild role is updated.
	RoleUpdate Signal[RoleUpdateEvent]
	// RoleUpdateBulk is emitted when multiple guild role updates are reported at once.
	RoleUpdateBulk Signal[RoleUpdateBulkEvent]
	// RoleDelete is emitted when a guild role is deleted.
	RoleDelete Signal[RoleDeleteEvent]
	// MemberAdd is emitted when a guild member joins.
	MemberAdd Signal[MemberAddEvent]
	// MemberUpdate is emitted when a guild member changes.
	MemberUpdate Signal[MemberUpdateEvent]
	/// MemberRemove is emitted when a guild member leaves.
	MemberRemove Signal[MemberRemoveEvent]
	// GuildEmojisUpdate is emitted when a guild's emojis are modified.
	GuildEmojisUpdate Signal[GuildEmojisUpdateEvent]
	// GuildStickersUpdate is emitted when a guild's stickers are modified.
	GuildStickersUpdate Signal[GuildStickersUpdateEvent]
	// GuildBanAdd is emitted when a user is banned from a guild.
	GuildBanAdd Signal[GuildBanAddEvent]
	// GuildBanRemove is emitted when a user is unbanned from a guild.
	GuildBanRemove Signal[GuildBanRemoveEvent]
	// UserUpdate is emitted when the current user changes.
	UserUpdate Signal[UserUpdateEvent]

	// See [Shard.Packet].
	ShardPacketReceived Signal[ShardPacketEvent]
	// See [Shard.Ready].
	ShardReady Signal[ShardReadyEvent]
	// See [Shard.Resumed].
	ShardResumed Signal[ShardResumedEvent]
	// See [Shard.Connected].
	ShardConnected Signal[ShardConnectedEvent]
	// See [Shard.Disconnected].
	ShardDisconnected Signal[ShardDisconnectedEvent]
	// See [Shard.Started].
	ShardStarted Signal[ShardStartedEvent]
	// See [Shard.Stopped].
	ShardStopped Signal[ShardStoppedEvent]
	// AllShardsStopped is emitted when there are no remaining running shards.
	AllShardsStopped Signal[AllShardsStoppedEvent]
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
	Message
}

// Guild returns the guild the message was sent in if it is cached.
func (e *MessageCreateEvent) Guild(cache *Cache) (Guild, bool) {
	if e.GuildID == nil {
		return Guild{}, false
	}

	return cache.Guilds.Get(*e.GuildID)
}

// Channel returns the channel the message was sent in if it is cached.
func (e *MessageCreateEvent) Channel(cache *Cache) (Channel, bool) {
	if e.GuildID != nil {
		guild, ok := cache.Guilds.Get(*e.GuildID)
		if !ok {
			return Channel{}, false
		}

		return guild.Channels.Get(e.ChannelID)
	} else {
		return cache.Channel(e.ChannelID)
	}
}

type MessageUpdateEvent struct {
	Shard   *Shard  `json:"-"`
	Member  *Member `json:"member"`
	GuildID *ID     `json:"guild_id"`
	Message
}

// Guild returns the guild the message was sent in if it was cached.
func (e *MessageUpdateEvent) Guild(cache *Cache) (Guild, bool) {
	return (*MessageCreateEvent)(e).Guild(cache)
}

// Channel returns the channel the message was sent in if it was cached.
func (e *MessageUpdateEvent) Channel(cache *Cache) (Channel, bool) {
	return (*MessageUpdateEvent)(e).Channel(cache)
}

type MessageDeleteEvent struct {
	Shard     *Shard   `json:"-"`
	GuildID   *ID      `json:"guild_id"`
	ChannelID ID       `json:"channel_id"`
	MessageID ID       `json:"id"`
	Content   *string  `json:"content"`
	AuthorID  *ID      `json:"author_id"`
	Member    *Member  `json:"member"`
	Cached    *Message `json:"-"`
}

type MessageDeleteBulkEvent struct {
	ChannelID ID
	GuildID   *ID
	Messages []BulkDeletedMessage
}

type BulkDeletedMessage struct {
	ID ID 
	// Cached is the message that was removed from cache by this event, if any.
	Cached *Message
}

// Guild returns the guild the message was sent in if it is cached.
func (e *MessageDeleteEvent) Guild(cache *Cache) (Guild, bool) {
	if e.GuildID == nil {
		return Guild{}, false
	}

	return cache.Guilds.Get(*e.GuildID)
}

// Channel returns the channel the message was sent in if it is cached.
func (e *MessageDeleteEvent) Channel(cache *Cache) (Channel, bool) {
	if e.GuildID != nil {
		guild, ok := cache.Guilds.Get(*e.GuildID)
		if !ok {
			return Channel{}, false
		}

		return guild.Channels.Get(e.ChannelID)
	} else {
		return cache.Channel(e.ChannelID)
	}
}

type TypingStartEvent struct {
	Shard     *Shard
	ChannelID ID
	UserID    ID
	Timestamp time.Time
	GuildID   *ID
	Member    *Member
}

// GuildAddEvent represents a guild becoming available or being joined.
type GuildAddEvent struct {
	Shard *Shard `json:"-"`
	Guild
}

// GuildUpdateEvent represents a guild being updated.
// The guild collections will only be present if the guild was already cached.
type GuildUpdateEvent struct {
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

type RoleCreateEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"-"`
	Role
}

// Guild returns the guild where the role is being added if it is cached.
func (e *RoleCreateEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

type RoleUpdateEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"-"`
	Role
}

// Guild returns the guild where the role is being updated if it is cached.
func (e *RoleUpdateEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

type RoleUpdateBulkEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"guild_id"`
	Roles   []Role `json:"roles"`
}

// Guild returns the guild where the role is being updated if it is cached.
func (e *RoleUpdateBulkEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

type RoleDeleteEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"guild_id"`
	RoleID  ID     `json:"role_id"`
	// Cached is the role that was removed from the cache by this event, if any.
	Cached *Role `json:"-"`
}

// Guild returns the guild where the role is being deleted if it is cached.
func (e *RoleDeleteEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

type MemberAddEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"guild_id"`
	Member
}

// Guild returns the guild where the member is being added if it is cached.
func (e *MemberAddEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

type MemberUpdateEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"guild_id"`
	Member
}

// Guild returns the guild where the member is being updated if it is cached.
func (e *MemberUpdateEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

type MemberRemoveEvent struct {
	Shard    *Shard
	GuildID  ID
	MemberID ID
	Cached   *Member
}

// Guild returns the guild where the member is being removed if it is cached.
func (e *MemberRemoveEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

type GuildEmojisUpdateEvent struct {
	Shard   *Shard       `json:"-"`
	GuildID ID           `json:"guild_id"`
	Emojis  []GuildEmoji `json:"emojis"`
}

// Guild returns the guild where the emojis are being updated if it is cached.
func (e *GuildEmojisUpdateEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

type GuildStickersUpdateEvent struct {
	Shard    *Shard         `json:"-"`
	GuildID  ID             `json:"guild_id"`
	Stickers []GuildSticker `json:"stickers"`
}

// Guild returns the guild where the stickers are being updated if it is cached.
func (e *GuildStickersUpdateEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

type GuildBanAddEvent struct {
	Shard   *Shard
	GuildID ID
	UserID  ID
}

// Guild returns the guild where the user was banned if it is cached.
func (e *GuildBanAddEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

// User returns the user that was banned if it is cached.
func (e *GuildBanAddEvent) User(cache *Cache) (User, bool) {
	return cache.Users.Get(e.UserID)
}

type GuildBanRemoveEvent struct {
	Shard   *Shard
	GuildID ID
	UserID  ID
}

// Guild returns the guild where the user was unbanned if it is cached.
func (e *GuildBanRemoveEvent) Guild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(e.GuildID)
}

// User returns the user that was unbanned if it is cached.
func (e *GuildBanRemoveEvent) User(cache *Cache) (User, bool) {
	return cache.Users.Get(e.UserID)
}

type UserUpdateEvent struct {
	Shard *Shard `json:"-"`
	UserPrivate
}

type AllShardsStoppedEvent struct {
	Gateway *Gateway
}

type shardEvents struct {
	// PacketReceived is emitted when a packet is received from Fluxer.
	PacketReceived Signal[ShardPacketEvent]
	// Ready is emitted when a READY packet is received.
	// This means the login was successful and contains various information, but no guilds will yet be available on a bot account.
	Ready Signal[ShardReadyEvent]
	// Resumed is emitted when a RESUMED packet is received.
	// This means a session was successfully resumed, but this won't always happen when reconnecting.
	// If resuming failed, a new session will be started which will cause Ready to be emitted again.
	Resumed Signal[ShardResumedEvent]
	// Connected is emitted when a websocket connection is established.
	Connected Signal[ShardConnectedEvent]
	// Disconnected is emitted when a websocket session ends.
	Disconnected Signal[ShardDisconnectedEvent]
	// Started is emitted when the shard starts.
	Started Signal[ShardStartedEvent]
	// Stopped is emitted when the shard stops.
	Stopped Signal[ShardStoppedEvent]
}

type ShardPacketEvent struct {
	Shard *Shard `json:"-"`
	GatewayPacket
}

type ShardReadyEvent struct {
	Shard     *Shard       `json:"-"`
	SessionID string       `json:"session_id"`
	User      UserPrivate  `json:"user"`
	Guilds    []ReadyGuild `json:"guilds"`
}

// ReadyGuild represents a guild in the READY payload which may or may not have its properties available.
type ReadyGuild struct {
	Unavailable bool
	ID          ID
	// Guild is the full guild if unavailable is false.
	Guild *Guild
	// Cached is the guild removed from cache if unavailable is true and it was cached.
	Cached *Guild
}

type ShardResumedEvent struct {
	Shard *Shard
}

type ShardConnectedEvent struct {
	Shard *Shard
}

type ShardDisconnectedEvent struct {
	Shard *Shard
	Err   error
	// Reconnecting is true if the shard will try to reconnect.
	Reconnecting bool
}

type ShardStartedEvent struct {
	Shard *Shard
}

type ShardStoppedEvent struct {
	Shard *Shard
	Err   error
}
