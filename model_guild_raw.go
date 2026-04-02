package flo

import "time"

type rawGuild struct {
	ID                    ID                         `json:"id"`
	Name                  string                     `json:"name"`
	Icon                  *string                    `json:"icon"`
	Banner                *string                    `json:"banner"`
	BannerWidth           *int                       `json:"banner_width"`
	BannerHeight          *int                       `json:"banner_height"`
	Splash                *string                    `json:"splash"`
	SplashWidth           *int                       `json:"splash_width"`
	SplashHeight          *int                       `json:"splash_height"`
	SplashCardAlignment   GuildSplashCardAlignment   `json:"splash_card_alignment"`
	EmbedSplash           *string                    `json:"embed_splash"`
	EmbedSplashWidth      *int                       `json:"embed_splash_width"`
	EmbedSplashHeight     *int                       `json:"embed_splash_height"`
	VanityURLCode         *string                    `json:"vanity_url_code"`
	OwnerID               ID                         `json:"owner_id"`
	SystemChannelID       *ID                        `json:"system_channel_id"`
	SystemChannelFlags    GuildSystemChannelFlags    `json:"system_channel_flags"`
	RulesChannelID        *ID                        `json:"rules_channel_id"`
	AFKChannelID          *ID                        `json:"afk_channel_id"`
	AFKTimeout            int                        `json:"afk_timeout"`
	Features              []GuildFeature             `json:"features"`
	VerifLevel            GuildVerifLevel            `json:"verification_level"`
	MFALevel              GuildMFALevel              `json:"mfa_level"`
	NSFWLevel             GuildNSFWLevel             `json:"nsfw_level"`
	ExplicitContentFilter GuildExplicitContentFilter `json:"explicit_content_filter"`
	DefaultMessageNotifs  UserNofifSettings          `json:"default_message_notifications"`
	DisabledOperations    GuildOperations            `json:"disabled_operations"`
	MessageHistoryCutoff  time.Time                  `json:"message_history_cutoff"`
}

func (g *Guild) update(guild rawGuild) {
	g.Name = guild.Name
	g.Icon = guild.Icon
	g.Banner = guild.Banner
	g.BannerWidth = guild.BannerWidth
	g.Splash = guild.Splash
	g.SplashWidth = guild.SplashWidth
	g.SplashHeight = guild.SplashHeight
	g.SplashCardAlignment = guild.SplashCardAlignment
	g.EmbedSplash = guild.EmbedSplash
	g.EmbedSplashWidth = guild.EmbedSplashWidth
	g.EmbedSplashHeight = guild.EmbedSplashHeight
	g.VanityURLCode = guild.VanityURLCode
	g.OwnerID = guild.OwnerID
	g.SystemChannelID = guild.SystemChannelID
	g.SystemChannelFlags = guild.SystemChannelFlags
	g.RulesChannelID = guild.RulesChannelID
	g.AFKChannelID = guild.AFKChannelID
	g.AFKTimeout = time.Duration(guild.AFKTimeout) * time.Second
	g.Features = guild.Features
	g.VerifLevel = guild.VerifLevel
	g.MFALevel = guild.MFALevel
	g.NSFWLevel = guild.NSFWLevel
	g.ExplicitContentFilter = guild.ExplicitContentFilter
	g.DefaultMessageNotifs = guild.DefaultMessageNotifs
	g.DisabledOperations = guild.DisabledOperations
	g.MessageHistoryCutoff = guild.MessageHistoryCutoff
}
