package flo

// Cache specifies caching targets and configuration.
// The zero value for Cache does not cache anything - use [NewCacheDefault] for generous defaults.
type Cache struct {
	// Guilds by default is populated by guilds which are available on the gateway.
	// If requesting a Guild over the REST API, you will manually need to add it if you wish to be able to retrieve it later.
	Guilds Collection[Guild]
	// UnavailableGuilds is populated by guilds have become unavailable but not been fully removed.
	UnavailableGuilds Collection[struct{}]
	// MakeGuild is used to create new guild entries if it is not nil.
	// This function should simply return a guild with the [Collection]s set for whatever caching is desired.
	MakeGuild func(ID) Guild
	// Users is populated by requested users or whatever is available from other gateway/REST paylods.
	Users Collection[User]
}

// NewCacheDefault returns a Cache which prioritises out-of-the-box usability.
// The caching here is quite aggressive but tuning it by modifying the fields is encouraged.
func NewCacheDefault() Cache {
	return Cache{
		Guilds:            NewCollectionUnlimited[Guild](),
		UnavailableGuilds: NewCollectionUnlimited[struct{}](),
		MakeGuild: func(id ID) Guild {
			channels := NewCollectionUnlimited[Channel]()
			roles := NewCollectionUnlimited[Role]()
			return Guild{
				Channels: &channels,
				Roles:    &roles,
			}
		},
		Users: NewCollectionUnlimited[User](),
	}
}
