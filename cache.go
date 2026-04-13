package flo

import "sync"

// Cache specifies caching targets and configuration.
// The zero value for Cache does not cache anything - use [NewCacheDefault] for generous defaults.
type Cache struct {
	// MakeChannel is used to create new channel entries if it is not nil.
	// This function should simply return a channel with the [Collection]s set for whatever limits are desired.
	MakeChannel func() Channel
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
	return Cache{
		MakeChannel: func() Channel {
			messages := NewCollection[Message](100)

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
