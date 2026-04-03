package flo

type rawUser struct {
	ID            ID        `json:"id"`
	Username      string    `json:"username"`
	Discriminator string    `json:"discriminator"`
	GlobalName    *string   `json:"global_name"`
	Avatar        *string   `json:"avatar"`
	AvatarColor   *ColorInt `json:"avatar_color"`
	Bot           bool      `json:"bot"`
	System        bool      `json:"system"`
	Flags         UserFlags `json:"flags"`
}

func cacheUser(cache Cache, user *rawUser) User {
	if cache.Users != nil {
		cache.Users.Set(user.ID, User(*user))
	}

	return User(*user)
}
