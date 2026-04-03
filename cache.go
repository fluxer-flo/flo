package flo

// Cache specifies caching targets and configuration.
// The zero value for Cache does not cache anything - use [DefaultCache] for generous defaults.
type Cache struct {
	// Guilds is used to cache guilds if it is not nil.
	Guilds Collection[Guild]
	// MakeGuild is used to create new guild entries if it is not nil.
	// This function should simply return a guild with the [Collection]s set for whatever caching is desired.
	// Without this the [Collection]s on the guild will be nil.
	MakeGuild func(ID) Guild
	// Users is used to cache users if it is not nil.
	Users Collection[User]
}

// NewDefaultCache returns a Cache which prioritises out-of-the-box usability.
func NewDefaultCache() Cache {
	return Cache{
		Guilds: NewCollection[Guild](),
		MakeGuild: func(id ID) Guild {
			return Guild{
				Channels: NewCollection[Channel](),
			}
		},
		Users: NewCollection[User](),
	}
}
