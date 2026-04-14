package flo

import (
	"context"
	"fmt"
	"math/big"
	"time"
)

//go:generate stringer -type=GuildSplashCardAlignment,GuildVerifLevel,GuildMFALevel,GuildNSFWLevel,GuildExplicitContentFilter -output=model_guild_string.go

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
	DefaultMessageNotifs  UserNotifSettings          `json:"default_message_notifications"`
	DisabledOperations    GuildOperations            `json:"disabled_operations"`
	MessageHistoryCutoff  *time.Time                 `json:"message_history_cutoff"`

	// Channels is the known channels of the guild.
	// If the guild is cached, they will be automatically updated.
	Channels *Collection[Channel] `json:"-"`
	// Roles is the known roles of the guild.
	// If the guild is cached, they will be automatically updated.
	Roles *Collection[Role] `json:"-"`
	// Members is the known members of the guild.
	// If the guild is cached, they will be automatically updated.
	Members *Collection[Member] `json:"-"`
	// Emojis is the known emojis of the guild.
	// If the guild is cached, they will be automatically updated.
	Emojis *Collection[GuildEmoji] `json:"-"`
	// Stickers is the known stickers of the guild.
	// If the guild is cached, they will be automatically updated.
	Stickers *Collection[GuildSticker] `json:"-"`
}

func (g *Guild) CreatedAt() time.Time {
	return g.ID.CreatedAt()
}

func (g *Guild) GetMembers(ctx context.Context, rest *REST, opts GetMembersOpts) ([]Member, error) {
	return rest.GetMembers(ctx, g.ID, opts)
}

func (g *Guild) RemoveMember(ctx context.Context, rest *REST, userID ID) error {
	return rest.RemoveMember(ctx, g.ID, userID)
}

func (g *Guild) RemoveMemberWithReason(ctx context.Context, rest *REST, userID ID, reason string) error {
	return rest.RemoveMemberWithReason(ctx, g.ID, userID, reason)
}

func (g *Guild) GetBans(ctx context.Context, rest *REST) ([]GuildBan, error) {
	return rest.GetGuildBans(ctx, g.ID)
}

func (g *Guild) CreateBan(ctx context.Context, rest *REST, userID ID, opts CreateGuildBanOpts) error {
	return rest.CreateGuildBan(ctx, g.ID, userID, opts)
}

func (g *Guild) RemoveBan(ctx context.Context, rest *REST, userID ID) error {
	return rest.RemoveGuildBan(ctx, g.ID, userID)
}

func (g *Guild) RemoveBanWithReason(ctx context.Context, rest *REST, userID ID, reason string) error {
	return rest.RemoveGuildBanWithReason(ctx, g.ID, userID, reason)
}

func (g *Guild) updateProperties(guild *Guild) {
	oldChannels := g.Channels
	oldRoles := g.Roles
	oldMember := g.Members
	oldEmojis := g.Emojis
	oldStickers := g.Stickers

	*g = *guild

	g.Channels = oldChannels
	g.Roles = oldRoles
	g.Members = oldMember
	g.Emojis = oldEmojis
	g.Stickers = oldStickers
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

type Role struct {
	ID            ID       `json:"id"`
	Name          string   `json:"name"`
	Color         ColorInt `json:"color"`
	Position      int      `json:"position"`
	HoistPosition *int     `json:"hoist_position"`
	Perms         Perms    `json:"permissions"`
	Hoist         bool     `json:"hoist"`
	Mentionable   bool     `json:"mentionable"`
	UnicodeEmoji  *string  `json:"unicode_emoji"`
}

func (r *Role) Mention() string {
	return fmt.Sprintf("<@&%d>", r.ID)
}

type Member struct {
	User              User       `json:"user"`
	Nick              *string    `json:"nick"`
	Avatar            *string    `json:"avatar"`
	Banner            *string    `json:"banner"`
	AccentColor       *ColorInt  `json:"accent_color"`
	Roles             []ID       `json:"roles"`
	JoinedAt          time.Time  `json:"joined_at"`
	Mute              bool       `json:"mute"`
	Deaf              bool       `json:"deaf"`
	CommDisabledUntil *time.Time `json:"communication_disabled_until"`
}

func (m *Member) ID() ID {
	return m.User.ID
}

func (m *Member) CreatedAt() time.Time {
	return m.User.CreatedAt()
}

func (m *Member) Mention() string {
	return m.User.Mention()
}

// DisplayName returns the member's rendered name in chat.
func (m *Member) DisplayName() string {
	if m.Nick != nil {
		return *m.Nick
	} else {
		return m.User.DisplayName()
	}
}

// Perms respresents a set of member permissions.
// The zero value can be used to represent no permissions.
type Perms struct {
	val *big.Int
}

var (
	PermCreateInstantInvite = NewPermsBit(0)
	PermKickMembers         = NewPermsBit(1)
	PermBanMembers          = NewPermsBit(2)
	PermAdministrator       = NewPermsBit(3)
	PermManageChannels      = NewPermsBit(4)
	PermManageGuild         = NewPermsBit(5)
	PermAddReactions        = NewPermsBit(6)
	PermViewAuditLog        = NewPermsBit(7)
	PermPrioritySpeaker     = NewPermsBit(8)
	PermStream              = NewPermsBit(9)
	PermViewChannel         = NewPermsBit(10)
	PermSendMessages        = NewPermsBit(11)
	PermSendTTSMessages     = NewPermsBit(12)
	PermManageMessages      = NewPermsBit(13)
	PermEmbedLinks          = NewPermsBit(14)
	PermAttachFiles         = NewPermsBit(15)
	PermReadMessageHistory  = NewPermsBit(16)
	PermMentionEveryone     = NewPermsBit(17)
	PermUseExternalEmojis   = NewPermsBit(18)
	PermConnect             = NewPermsBit(20)
	PermSpeak               = NewPermsBit(21)
	PermMuteMembers         = NewPermsBit(22)
	PermDeafenMembers       = NewPermsBit(23)
	PermMoveMembers         = NewPermsBit(24)
	PermUseVAD              = NewPermsBit(25)
	PermChangeNickname      = NewPermsBit(26)
	PermManageNicknames     = NewPermsBit(27)
	PermManageRoles         = NewPermsBit(28)
	PermManageWebhooks      = NewPermsBit(29)
	PermManageExpressions   = NewPermsBit(30)
	PermUseExternalStickers = NewPermsBit(37)
	PermModerateMembers     = NewPermsBit(40)
	PermCreateExpressions   = NewPermsBit(43)
	PermPinMembers          = NewPermsBit(51)
	PermBypassSlowmode      = NewPermsBit(52)
	PermUpdateRTCRegion     = NewPermsBit(53)
)

// NewPerms creates permissions as a combination of all provided permissions.
func NewPerms(p ...Perms) Perms {
	var result Perms

	for _, perms := range p {
		result = result.Union(perms)
	}

	return result
}

func NewPermsBit(bit uint) Perms {
	result := big.NewInt(1)
	result.Lsh(result, bit)

	return Perms{result}
}

// PermsFromBigInt creates permissions with the underlying [big.Int] representation.
func PermsFromBigInt(val *big.Int) Perms {
	if val == nil {
		return Perms{}
	}

	var result Perms
	result.val = big.NewInt(0)
	result.val.Set(val)

	return result
}

// BigInt returns the underlying [big.Int] representation of the permissions.
func (p Perms) BigInt() *big.Int {
	result := big.NewInt(0)

	if p.val != nil {
		result.Set(p.val)
	}

	return result
}

// Intersection returns the common permissions between p and p2.
func (p Perms) Intersection(p2 Perms) Perms {
	if p.val == nil || p2.val == nil {
		return Perms{}
	}

	result := big.NewInt(0)
	result.And(p.val, p2.val)

	return Perms{result}
}

// Union returns a combination of permissions between p and p2.
func (p Perms) Union(p2 Perms) Perms {
	if p.val == nil {
		return p2
	}
	if p2.val == nil {
		return p
	}

	result := big.NewInt(0)
	result.Or(p.val, p2.val)

	return Perms{result}
}

// Difference returns the set of permissions which are in p but not in p2.
func (p Perms) Difference(p2 Perms) Perms {
	if p.val == nil {
		return Perms{}
	}
	if p2.val == nil {
		return p
	}

	mask := big.NewInt(0)
	mask.Not(p2.val)

	result := big.NewInt(0)
	result.And(result, mask)

	return Perms{result}
}

// Equal returns true if p and p2 represent the same set of permissions.
func (p Perms) Equal(p2 Perms) bool {
	if p.val == nil {
		return p2.val == nil || p2.val.BitLen() == 0
	}
	if p2.val == nil {
		return p.val == nil || p.val.BitLen() == 0
	}

	return p.val.Cmp(p2.val) == 0
}

// Has returns true if p contains all the perms in p2.
func (p Perms) Has(p2 ...Perms) bool {
	other := NewPerms(p2...)
	return p.Intersection(other).Equal(other)
}

// Set adds all of the permissions on p2 to p.
func (p *Perms) Set(p2 ...Perms) {
	*p = p.Union(NewPerms(p2...))
}

// Clear removes all of the permissions on p2 from p.
// If p2 is empty, p is fully cleared instead.
func (p *Perms) Clear(p2 ...Perms) {
	if len(p2) == 0 {
		p.val = nil
		return
	}

	*p = p.Difference(NewPerms(p2...))
}

func (p Perms) String() string {
	if p.val == nil {
		return "0"
	} else {
		return p.val.String()
	}
}

func (p Perms) MarshalJSON() ([]byte, error) {
	return fmt.Appendf(nil, `"%s"`, p), nil
}

func (p *Perms) UnmarshalJSON(data []byte) error {
	if len(data) < 2 || data[0] != '"' || data[len(data)-1] != '"' {
		return fmt.Errorf("expected JSON string")
	}

	unquoted := data[1 : len(data)-1]

	result := big.NewInt(0)
	err := result.UnmarshalText(unquoted)
	if err != nil {
		return err
	}

	p.val = result
	return nil
}

// GuildEmoji represents a custom emoji in a guild.
type GuildEmoji struct {
	ID       ID     `json:"id"`
	Name     string `json:"name"`
	Animated bool   `json:"animated"`
	// User is the user who uploaded the emoji, which may not be available in all responses.
	User *User `json:"user"`
}

func (e *GuildEmoji) CreatedAt() time.Time {
	return e.ID.CreatedAt()
}

// Render creates a string that can be used to display the emoji in chat.
func (e *GuildEmoji) Render() string {
	if !e.Animated {
		return fmt.Sprintf("<:%s:%d>", e.Name, e.ID)
	} else {
		return fmt.Sprintf("<a:%s:%d>", e.Name, e.ID)
	}
}

func (e *GuildEmoji) updateWithoutUser(emoji *GuildEmoji) {
	oldUser := e.User
	*e = *emoji
	e.User = oldUser
}

// GuildSticker reperesents a custom sticker in a guild.
type GuildSticker struct {
	ID          ID       `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Animated    bool     `json:"animated"`
	// User is the user who uploaded the emoji, which may not be available in all responses.
	User *User `json:"user"`
}

// GuildBan represents a member ban in a guild.
type GuildBan struct {
	User        User       `json:"user"`
	Reason      string     `json:"reason"`
	ModeratorID ID         `json:"moderator_id"`
	BannedAt    time.Time  `json:"banned_at"`
	ExpiresAt   *time.Time `json:"expires_at"`
}
