package flo

import "time"

type Guild struct {
	ID                    ID
	Name                  string
	Icon                  *string
	Banner                *string
	BannerWidth           *int
	BannerHeight          *int
	Splash                *string
	SplashWidth           *int
	SplashHeight          *int
	SplashCardAlignment   GuildSplashCardAlignment
	EmbedSplash           *string
	EmbedSplashWidth      *int
	EmbedSplashHeight     *int
	VanityURLCode         *string
	OwnerID               ID
	SystemChannelID       *ID
	SystemChannelFlags    GuildSystemChannelFlags
	RulesChannelID        *ID
	AFKChannelID          *ID
	AFKTimeout            time.Duration
	Features              []GuildFeature
	VerifLevel            GuildVerifLevel
	MFALevel              GuildMFALevel
	NSFWLevel             GuildNSFWLevel
	ExplicitContentFilter GuildExplicitContentFilter
	DefaultMessageNotifs  UserNofifSettings
	DisabledOperations    GuildOperations
	MessageHistoryCutoff  time.Time

	Channels Collection[Channel]
}

func (g *Guild) CreatedAt() time.Time {
	return g.ID.CreatedAt()
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
