package flo

// Cache specifies caching targets and configuration.
// The zero value for Cache does not cache anything - use [NewCacheDefault] for generous defaults.
type Cache struct {
	Guilds Collection[Guild]
	// UnavailableGuilds represents the set of guilds which have become unavailable but not been fully removed.
	UnavailableGuilds Collection[struct{}]
	// MakeGuild is used to create new guild entries if it is not nil.
	// This function should simply return a guild with the [Collection]s set for whatever caching is desired.
	MakeGuild func(ID) Guild
	Users     Collection[User]
}

// NewCacheDefault returns a Cache which prioritises out-of-the-box usability.
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
