package flo

import "sync"

// Cache specifies caching targets and configuration.
// The zero value for Cache does not cache anything - use [NewCacheDefault] for generous defaults.
type Cache struct {
	// DMChannels is populated by channels of type [ChannelTypeDM].
	DMChannels Collection[Channel]
	// MakeDMChannel is used to create new DM channel entries if it is not nil.
	// This function should simply return a channel with the [Collection]s set for whatever limits are desired.
	MakeDMChannel func() Channel
	// DMChannels is populated by channels of type [ChannelTypeGroupDM].
	GroupDMChannels Collection[Channel]
	// MakeGroupDM is used to create new group DM channel entries if it is not nil.
	// This function should simply return a channel with the [Collection]s set for whatever limits are desired.
	MakeGroupDMChannel func() Channel
	// ChannelGuilds is populated with a mapping from guild channel ID -> guild ID.
	ChannelGuilds Collection[ID]
	// MakePrivateChannel is used to create new guild channel entries if it is not nil.
	// This function should simply return a channel with the [Collection]s set for whatever limits are desired.
	MakeGuildChannel func() Channel
	// Guilds by default is populated by guilds which are available on the gateway.
	// If requesting a Guild over the REST API, you will manually need to add it if you wish to be able to retrieve it later.
	Guilds Collection[Guild]
	// UnavailableGuilds is populated by guilds have become unavailable but not been fully removed.
	UnavailableGuilds Collection[struct{}]
	// MakeGuild is used to create new guild entries if it is not nil.
	// This function should simply return a guild with the [Collection]s set for whatever limits are desired.
	MakeGuild func() Guild
	// Users is populated by requested users or whatever is available from other gateway/REST paylods.
	Users Collection[User]
	// CacheCurrentUser is used to determine whether to cache the [UserPrivate] object for the authenticated user.
	CacheCurrentUser bool
	currentUserMu    sync.RWMutex
	hasCurrentUser   bool
	currentUser      UserPrivate
}

// NewCacheDefault returns a Cache which prioritises out-of-the-box usability.
// The caching here is quite aggressive but tuning it by modifying the fields is encouraged.
func NewCacheDefault() Cache {
	const messageCount = 100

	return Cache{
		DMChannels:      NewCollection[Channel](300),
		GroupDMChannels: NewCollectionUnlimited[Channel](),
		MakeDMChannel: func() Channel {
			messages := NewCollection[Message](messageCount)

			return Channel{
				Messages: &messages,
			}
		},
		MakeGroupDMChannel: func() Channel {
			messages := NewCollection[Message](messageCount)

			return Channel{
				Messages: &messages,
			}
		},
		MakeGuildChannel: func() Channel {
			messages := NewCollection[Message](messageCount)

			return Channel{
				Messages: &messages,
			}
		},
		Guilds:            NewCollectionUnlimited[Guild](),
		UnavailableGuilds: NewCollectionUnlimited[struct{}](),
		MakeGuild: func() Guild {
			channels := NewCollectionUnlimited[Channel]()
			roles := NewCollectionUnlimited[Role]()
			members := NewCollectionUnlimited[Member]()
			emojis := NewCollectionUnlimited[GuildEmoji]()
			stickers := NewCollectionUnlimited[GuildSticker]()

			return Guild{
				Channels: &channels,
				Roles:    &roles,
				Members:  &members,
				Emojis:   &emojis,
				Stickers: &stickers,
			}
		},
		Users:            NewCollectionUnlimited[User](),
		CacheCurrentUser: true,
	}
}

// Channel looks up a channel by its ID alone.
// Guild channels will not be found if ChannelGuilds does not contain them.
// This should be used with caution - if you, for example, provide a command that allows any channel ID and looks it up with this method it could easily lead to privilege escalation!
// If you only want channels from a specific guild, first look it up in Guilds then look it up on [Guild.Channels] if it is present.
func (c *Cache) Channel(channelID ID) (Channel, bool) {
	if guildID, ok := c.ChannelGuilds.Get(channelID); ok {
		guild, ok := c.Guilds.Get(guildID)
		if ok {
			return guild.Channels.Get(channelID)
		}
	}

	if channel, ok := c.DMChannels.Get(channelID); ok {
		return channel, true
	}

	if channel, ok := c.GroupDMChannels.Get(channelID); ok {
		return channel, true
	}

	return Channel{}, false

}

func (c *Cache) UpdateChannel(channelID ID, update func(channel *Channel)) bool {
	if guildID, ok := c.ChannelGuilds.Get(channelID); ok {
		guild, ok := c.Guilds.Get(guildID)
		if ok && guild.Channels != nil {
			return guild.Channels.Update(channelID, update)
		}
	}

	if c.DMChannels.Update(channelID, update) {
		return true
	}

	if c.GroupDMChannels.Update(channelID, update) {
		return true
	}

	return false
}

func (c *Cache) CurrentUser() (UserPrivate, bool) {
	c.currentUserMu.RLock()
	defer c.currentUserMu.RUnlock()

	if c.CacheCurrentUser {
		return c.currentUser, true
	} else {
		return UserPrivate{}, false
	}
}

func (c *Cache) ExpectCurrentUser() UserPrivate {
	user, ok := c.CurrentUser()
	if !ok {
		panic("expected CurrentUser() to be present")
	}

	return user
}

func (c *Cache) ClearCurrentUser() {
	c.currentUserMu.Lock()
	defer c.currentUserMu.Unlock()

	c.currentUser = UserPrivate{}
	c.hasCurrentUser = false
}

func (c *Cache) UpdateCurrentUser(user UserPrivate) {
	c.currentUserMu.Lock()
	defer c.currentUserMu.Unlock()

	if c.CacheCurrentUser {
		c.hasCurrentUser = true
		c.currentUser = user
	}
}

// that's all folks!
// (internal caching functions below)

func initGuildChannelCollections(channel *Channel, cache *Cache) {
	var template Channel
	if cache.MakeGuildChannel != nil {
		template = cache.MakeGuildChannel()
	}

	channel.Messages = template.Messages
}

func initDMChannelCollections(channel *Channel, cache *Cache) {
	var template Channel
	if cache.MakeDMChannel != nil {
		template = cache.MakeDMChannel()
	}

	channel.Messages = template.Messages
}

func initGroupDMChannelCollections(channel *Channel, cache *Cache) {
	var template Channel
	if cache.MakeDMChannel != nil {
		template = cache.MakeDMChannel()
	}

	channel.Messages = template.Messages
}

func cacheChannel(channel *Channel, cache *Cache) {
	if cache == nil {
		return
	}

	if channel.GuildID != nil {
		guild, ok := cache.Guilds.Get(*channel.GuildID)
		if ok {
			cacheGuildChannel(&guild, channel, cache)
		}
	} else if channel.Type == ChannelTypeDM {
		initDMChannelCollections(channel, cache)

		cache.DMChannels.Upsert(channel.ID, *channel, func(cached *Channel) {
			cached.updateProperties(channel)
			*channel = *cached
		})

		for _, recipient := range channel.Recipients {
			cache.Users.Set(recipient.ID, recipient)
		}
	} else if channel.Type == ChannelTypeGroupDM {
		initGroupDMChannelCollections(channel, cache)

		cache.GroupDMChannels.Upsert(channel.ID, *channel, func(cached *Channel) {
			cached.updateProperties(channel)
			*channel = *cached
		})

		for _, recipient := range channel.Recipients {
			cache.Users.Set(recipient.ID, recipient)
		}
	}
}

func cacheGuildChannel(guild *Guild, channel *Channel, cache *Cache) {
	if cache == nil {
		return
	}

	if guild.Channels == nil {
		return
	}

	initGuildChannelCollections(channel, cache)

	guild.Channels.Upsert(channel.ID, *channel, func(cached *Channel) {
		cached.updateProperties(channel)
		*channel = *cached
	})
}

func uncacheChannel(channel *Channel, cache *Cache) {
	if cache == nil {
		return
	}

	if channel.GuildID != nil {
		guild, ok := cache.Guilds.Get(*channel.GuildID)
		if ok {
			guild.Channels.Delete(channel.ID)
		}

		cache.ChannelGuilds.Delete(channel.ID)
	} else if channel.Type == ChannelTypeDM {
		cache.DMChannels.Delete(channel.ID)
	} else if channel.Type == ChannelTypeGroupDM {
		cache.GroupDMChannels.Delete(channel.ID)
	}
}

func cacheGatewayMessage(msg *MessageCreateEvent, isNew bool, cache *Cache) {
	if cache == nil {
		return
	}

	cacheMessageCommon(&msg.Message, cache)

	updateChannel := func(channel *Channel) {
		if isNew {
			lastMsgID := msg.ID
			channel.LastMessageID = &lastMsgID
			channel.Messages.Set(msg.ID, msg.Message)
		} else {
			channel.Messages.Upsert(msg.ID, msg.Message, func(cached *Message) {
				msg.Nonce = cached.Nonce
				*cached = msg.Message
			})
		}
	}

	if msg.GuildID != nil {
		if guild, ok := cache.Guilds.Get(*msg.GuildID); ok {
			if msg.Member != nil && guild.Members != nil {
				guild.Members.Set(msg.Member.ID(), *msg.Member)
			}

			if guild.Channels != nil {
				guild.Channels.Update(msg.ChannelID, updateChannel)
			}
		}
	} else {
		cache.UpdateChannel(msg.ChannelID, updateChannel)
	}
}

func cacheMessage(msg *Message, cache *Cache) {
	if cache == nil {
		return
	}

	cacheMessageCommon(msg, cache)

	channel, ok := cache.Channel(msg.ChannelID)
	if !ok {
		return
	}

	channel.Messages.Upsert(msg.ID, *msg, func(cached *Message) {
		msg.Nonce = cached.Nonce
		*cached = *msg
	})
}

func cacheMessages(channelID ID, messages []Message, cache *Cache) {
	if cache == nil {
		return
	}

	for _, msg := range messages {
		cacheMessageCommon(&msg, cache)
	}

	channel, ok := cache.Channel(channelID)
	if !ok {
		return
	}

	for i := range messages {
		msg := &messages[i]

		channel.Messages.Upsert(msg.ID, *msg, func(cached *Message) {
			msg.Nonce = cached.Nonce
			*cached = *msg
		})
	}

}

func cacheMessageCommon(msg *Message, cache *Cache) {
	if msg.WebhookID == nil {
		cache.Users.Set(msg.Author.ID, msg.Author)
	}

	for _, user := range msg.Mentions {
		cache.Users.Set(user.ID, user)
	}

	if msg.ReferencedMessage != nil {
		referenced := *msg.ReferencedMessage
		// NOTE: prevent recursion, just in case
		referenced.ReferencedMessage = nil
		cacheMessage(&referenced, cache)
	}
}

func uncacheMessage(channelID ID, msgID ID, cache *Cache) {
	channel, ok := cache.Channel(channelID)
	if !ok {
		return
	}

	if channel.Messages == nil {
		return
	}

	channel.Messages.Delete(msgID)
}

func uncacheMessages(channelID ID, messageIDs []ID, cache *Cache) {
	channel, ok := cache.Channel(channelID)

	if !ok {
		return
	}

	if channel.Messages == nil {
		return
	}

	for _, id := range messageIDs {
		channel.Messages.Delete(id)
	}
}

func uncacheGatewayMessage(msg *MessageDeleteEvent, cache *Cache) *Message {
	if msg.GuildID != nil {
		if guild, ok := cache.Guilds.Get(*msg.GuildID); ok {
			if channel, ok := guild.Channels.Get(msg.ChannelID); ok {
				msg, _ := channel.Messages.Delete(msg.MessageID)
				return msg
			}
		}
	} else if channel, ok := cache.Channel(msg.ChannelID); ok {
		msg, _ := channel.Messages.Delete(msg.MessageID)
		return msg
	}

	return nil
}

func cacheGuild(guild *Guild, cache *Cache) {
	if cache == nil {
		return
	}

	cache.Guilds.Update(guild.ID, func(cached *Guild) {
		cached.updateProperties(guild)
		*guild = *cached
	})
}

func uncacheGuild(guildID ID, cache *Cache) *Guild {
	if cache == nil {
		return nil
	}

	removed, _ := cache.Guilds.Delete(guildID)
	cache.UnavailableGuilds.Set(guildID, struct{}{})

	if removed != nil {
		for channelID := range removed.Channels.IDs() {
			cache.ChannelGuilds.Delete(channelID)
		}
	}

	return removed
}

func initGuildCollections(guild *Guild, cache *Cache) {
	if cache == nil {
		channels := NewCollectionUnlimited[Channel]()
		guild.Channels = &channels

		members := NewCollectionUnlimited[Member]()
		guild.Members = &members

		roles := NewCollectionUnlimited[Role]()
		guild.Roles = &roles

		emojis := NewCollectionUnlimited[GuildEmoji]()
		guild.Emojis = &emojis

		stickers := NewCollectionUnlimited[GuildSticker]()
		guild.Stickers = &stickers
	} else {
		var template Guild
		if cache.MakeGuild != nil {
			template = cache.MakeGuild()
		}

		guild.Channels = template.Channels
		guild.Members = template.Members
		guild.Roles = template.Roles
		guild.Emojis = template.Emojis
		guild.Stickers = template.Stickers
	}
}

func cacheGatewayGuild(guild *gatewayGuild, cache *Cache) (Guild, bool) {
	result := guild.Properties
	initGuildCollections(&result, cache)

	for _, channel := range guild.Channels {
		cacheGuildChannel(&result, &channel, cache)
	}

	for _, role := range guild.Roles {
		result.Roles.Set(role.ID, role)
	}

	for _, member := range guild.Members {
		cacheGuildMember(&result, &member, cache)
	}

	for _, emoji := range guild.Emojis {
		result.Emojis.Set(emoji.ID, emoji)
	}

	for _, sticker := range guild.Stickers {
		result.Stickers.Set(sticker.ID, sticker)
	}

	var wasUnavailable bool
	if cache != nil {
		cache.Guilds.Set(result.ID, result)
		_, wasUnavailable = cache.UnavailableGuilds.Delete(result.ID)
	}

	return result, wasUnavailable
}

func cacheMember(guildID ID, member *Member, cache *Cache) {
	if cache == nil {
		return
	}

	cacheMemberCommon(member, cache)

	guild, ok := cache.Guilds.Get(guildID)
	if !ok {
		return
	}

	if guild.Members == nil {
		return
	}

	guild.Members.Set(member.ID(), *member)
}

func cacheMembers(guildID ID, members []Member, cache *Cache) {
	if cache == nil {
		return
	}

	for _, member := range members {
		cacheMemberCommon(&member, cache)
	}

	guild, ok := cache.Guilds.Get(guildID)
	if !ok {
		return
	}

	if guild.Members == nil {
		return
	}

	for _, member := range members {
		guild.Members.Set(member.ID(), member)
	}
}

func cacheGuildMember(guild *Guild, member *Member, cache *Cache) {
	cacheMemberCommon(member, cache)

	if guild.Members != nil {
		guild.Members.Set(member.ID(), *member)
	}
}

func cacheMemberCommon(member *Member, cache *Cache) {
	cache.Users.Set(member.ID(), member.User)
}

func uncacheMember(guildID ID, memberID ID, cache *Cache) *Member {
	if cache == nil {
		return nil
	}

	guild, ok := cache.Guilds.Get(guildID)
	if !ok {
		return nil
	}

	if guild.Members == nil {
		return nil
	}

	member, _ := guild.Members.Delete(memberID)
	return member
}

func cacheCurrentUser(user *UserPrivate, cache *Cache) {
	if cache == nil {
		return
	}

	cache.UpdateCurrentUser(*user)
	cache.Users.Set(user.ID, user.User)
}
