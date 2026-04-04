package flo

import "time"

type Guild struct {
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
	AFKTimeoutSecs        int                        `json:"afk_timeout"`
	Features              []GuildFeature             `json:"features"`
	VerifLevel            GuildVerifLevel            `json:"verification_level"`
	MFALevel              GuildMFALevel              `json:"mfa_level"`
	NSFWLevel             GuildNSFWLevel             `json:"nsfw_level"`
	ExplicitContentFilter GuildExplicitContentFilter `json:"explicit_content_filter"`
	DefaultMessageNotifs  UserNofifSettings          `json:"default_message_notifs"`
	DisabledOperations    GuildOperations            `json:"disabled_operations"`
	MessageHistoryCutoff  time.Time                  `json:"message_history_cutoff"`

	Channels *Collection[Channel] `json:"-"`
}

func (g *Guild) CreatedAt() time.Time {
	return g.ID.CreatedAt()
}

func (g *Guild) updateREST(guild *Guild) {
	oldChannels := g.Channels
	*g = *guild
	g.Channels = oldChannels
}

type GuildSplashCardAlignment uint

const (
	GuildSplashCardAlignCenter GuildSplashCardAlignment = 0
	GuildSplashCardAlignLeft   GuildSplashCardAlignment = 1
	GuildSplashCardAlignRight  GuildSplashCardAlignment = 2
)

type GuildSystemChannelFlags uint

const (
	SystemChannelFlagSupressJoinNotifications GuildSystemChannelFlags = 1 << 0
)

type GuildFeature string

const (
	GuildFeatureAnimatedIcon                   GuildFeature = "ANIMATED_ICON"
	GuildFeatureAnimatedBanner                 GuildFeature = "ANIMATED_BANNER"
	GuildFeatureBanner                         GuildFeature = "BANNER"
	GuildFeatureDetachedBanner                 GuildFeature = "DETACHED_BANNER"
	GuildFeatureInviteSplash                   GuildFeature = "INVITE_SPLASH"
	GuildFeatureInvitesDisabled                GuildFeature = "INVITES_DISABLED"
	GuildFeatureTextChannelFlexibleNames       GuildFeature = "TEXT_CHANNEL_FLEXIBLE_NAMES"
	GuildFeatureMoreEmoji                      GuildFeature = "MORE_EMOJI"
	GuildFeatureMoreStickers                   GuildFeature = "MORE_STICKERS"
	GuildFeatureUnlimitedEmoji                 GuildFeature = "UNLIMITED_EMOJI"
	GuildFeatureUnlimitedStickers              GuildFeature = "UNLIMITED_STICKERS"
	GuildFeatureExpressionPurgeAllowed         GuildFeature = "EXPRESSION_PURGE_ALLOWED"
	GuildFeatureVanityURL                      GuildFeature = "VANITY_URL"
	GuildFeatureDiscoverable                   GuildFeature = "DISCOVERABLE"
	GuildFeaturePartnered                      GuildFeature = "PARTNERED"
	GuildFeatureVerified                       GuildFeature = "VERIFIED"
	GuildFeatureVIPVoice                       GuildFeature = "VIP_VOICE"
	GuildFeatureUnavailableForEveryone         GuildFeature = "UNAVAILABLE_FOR_EVERYONE"
	GuildFeatureUnavailableForEveryoneButStaff GuildFeature = "UNAVAILABLE_FOR_EVERYONE_BUT_STAFF"
	GuildFeatureVisionary                      GuildFeature = "VISIONARY"
	GuildFeatureOperator                       GuildFeature = "OPERATOR"
	GuildFeatureLargeGuildOverride             GuildFeature = "LARGE_GUILD_OVERRIDE"
	GuildFeatureVeryLargeGuild                 GuildFeature = "VERY_LARGE_GUILD"
)

type GuildVerifLevel uint

const (
	GuildVerifLevelNone      GuildVerifLevel = 0
	GuildVerifLevelLow       GuildVerifLevel = 1
	GuildVerifLevelMedium    GuildVerifLevel = 2
	GuildVerifLevelHigh      GuildVerifLevel = 3
	GuildVerifLevelVeryHeigh GuildVerifLevel = 4
)

type GuildMFALevel uint

const (
	GuildMFALevelNone     GuildMFALevel = 0
	GuildMFALevelElevated GuildMFALevel = 1
)

type GuildNSFWLevel uint

const (
	GuildNSFWLevelDefault       GuildNSFWLevel = 0
	GuildNSFWLevelExplicit      GuildNSFWLevel = 1
	GuildNSFWLevelSafe          GuildNSFWLevel = 2
	GuildNSFWLevelAgeRestricted GuildNSFWLevel = 3
)

type GuildExplicitContentFilter uint

const (
	GuildExplicitContentFilterDisabled            GuildExplicitContentFilter = 0
	GuildExplicitContentFilterMembersWithoutRoles GuildExplicitContentFilter = 1
	GuildExplicitContentFilterAllMembers          GuildExplicitContentFilter = 2
)

type GuildOperations uint

const (
	GuildOperationPushNotifications GuildOperations = 1 << 0
	GuildOperationEveryoneMentions  GuildOperations = 1 << 1
	GuildOperationTypingEvents      GuildOperations = 1 << 2
	GuildOperationInstantInvites    GuildOperations = 1 << 3
	GuildOperationSendMessage       GuildOperations = 1 << 4
	GuildOperationReactions         GuildOperations = 1 << 5
	GuildOperationMemberListUpdates GuildOperations = 1 << 6
)
