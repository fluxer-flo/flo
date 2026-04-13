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

	// See [Shard.PacketReceived].
	ShardPacketReceived Signal[ShardPacketEvent]
	// See [Shard.Ready].
	ShardReady Signal[ShardReadyEvent]
	// See [Shard.Resumed].
	ShardResumed Signal[ShardResumeEvent]
	// See [Shard.Disconnected].
	ShardDisconnected Signal[ShardDisconnectEvent]
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

type MessageUpdateEvent struct {
	Shard   *Shard  `json:"-"`
	Member  *Member `json:"member"`
	GuildID *ID     `json:"guild_id"`
	Message
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

type RoleUpdateEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"-"`
	Role
}

type RoleUpdateBulkEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"guild_id"`
	Roles   []Role `json:"roles"`
}

type RoleDeleteEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"guild_id"`
	RoleID  ID     `json:"role_id"`
	// Cached is the role that was removed from the cache by this event, if any.
	Cached *Role `json:"-"`
}

type MemberAddEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"guild_id"`
	Member
}

type MemberUpdateEvent struct {
	Shard   *Shard `json:"-"`
	GuildID ID     `json:"guild_id"`
	Member
}

type MemberRemoveEvent struct {
	Shard    *Shard
	GuildID  ID
	MemberID ID
	Cached   *Member
}

type GuildEmojisUpdateEvent struct {
	Shard   *Shard       `json:"-"`
	GuildID ID           `json:"guild_id"`
	Emojis  []GuildEmoji `json:"emojis"`
}

type GuildStickersUpdateEvent struct {
	Shard    *Shard         `json:"-"`
	GuildID  ID             `json:"guild_id"`
	Stickers []GuildSticker `json:"stickers"`
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
	Resumed Signal[ShardResumeEvent]
	// Disconnected is emitted when a websocket session ends.
	Disconnected Signal[ShardDisconnectEvent]
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
}

type ShardResumeEvent struct {
	Shard *Shard
}

type ShardDisconnectEvent struct {
	Shard *Shard
	Err   error
	// Reconnecting is true if the shard will try to reconnect.
	Reconnecting bool
}
