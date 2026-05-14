package flo

import (
	"context"
	"time"
)

type InviteType uint

const (
	InviteTypeGuild       InviteType = 0
	InviteTypeGroupDM     InviteType = 1
	InviteTypeEmojiPack   InviteType = 2
	InviteTypeStickerPack InviteType = 3
)

// Invite represents an invitation to a guild, group DM or sticker/emoji pack.
type Invite struct {
	Code          string         `json:"code"`
	Type          InviteType     `json:"type"`
	Inviter       *User          `json:"inviter"`
	ExpiresAt     *time.Time     `json:"expires_at"`
	TempAccess    bool           `json:"temporary"`
	Guild         *InviteGuild   `json:"guild"`
	Channel       *InviteChannel `json:"channel"`
	MemberCount   *int           `json:"member_count"`
	PresenceCount *int           `json:"presence_count"`
	Uses          *int           `json:"uses"`
	MaxUses       *int           `json:"max_uses"`
	MaxAgeSecs    *int           `json:"max_age"`
	CreatedAt     *time.Time     `json:"created_at"`
}

// ToGuildInvite returns a [GuildInvite] if the invite can be converted to it.
func (i Invite) ToGuildInvite() (GuildInvite, bool) {
	if i.Type != InviteTypeGuild {
		return GuildInvite{}, false
	}

	if i.Guild == nil ||
		i.Channel == nil ||
		i.MemberCount == nil ||
		i.PresenceCount == nil {
		return GuildInvite{}, false
	}

	return GuildInvite{
		Code:          i.Code,
		Inviter:       i.Inviter,
		ExpiresAt:     i.ExpiresAt,
		TempAccess:    i.TempAccess,
		Guild:         *i.Guild,
		Channel:       *i.Channel,
		MemberCount:   *i.MemberCount,
		PresenceCount: *i.PresenceCount,
		Uses:          i.Uses,
		MaxUses:       i.MaxUses,
		MaxAgeSecs:    i.MaxAgeSecs,
		CreatedAt:     i.CreatedAt,
	}, true
}

func (i Invite) ToGroupDMInvite() (GroupDMInvite, bool) {
	if i.Type != InviteTypeGroupDM {
		return GroupDMInvite{}, false
	}

	if i.Channel == nil ||
		i.MemberCount == nil {
		return GroupDMInvite{}, false
	}

	return GroupDMInvite{
		Code:        i.Code,
		Inviter:     i.Inviter,
		ExpiresAt:   i.ExpiresAt,
		TempAccess:  i.TempAccess,
		Channel:     *i.Channel,
		MemberCount: *i.MemberCount,
	}, true
}

func (i Invite) Delete(ctx context.Context, rest *REST) error {
	return rest.DeleteInvite(ctx, i.Code)
}

func (i Invite) DeleteWithReason(ctx context.Context, rest *REST, reason string) error {
	return rest.DeleteInviteWithReason(ctx, i.Code, reason)
}

// GuildInvite represents an invitation to a guild.
type GuildInvite struct {
	Code          string        `json:"code"`
	Inviter       *User         `json:"inviter"`
	ExpiresAt     *time.Time    `json:"expires_at"`
	TempAccess    bool          `json:"temporary"`
	Guild         InviteGuild   `json:"guild"`
	Channel       InviteChannel `json:"channel"`
	MemberCount   int           `json:"member_count"`
	PresenceCount int           `json:"presence_count"`
	Uses          *int          `json:"uses"`
	MaxUses       *int          `json:"max_uses"`
	MaxAgeSecs    *int          `json:"max_age"`
	CreatedAt     *time.Time    `json:"created_at"`
}

func (i GuildInvite) ToInvite() Invite {
	return Invite{
		Code:          i.Code,
		Type:          InviteTypeGuild,
		Inviter:       i.Inviter,
		ExpiresAt:     i.ExpiresAt,
		TempAccess:    i.TempAccess,
		Guild:         &i.Guild,
		Channel:       &i.Channel,
		MemberCount:   &i.MemberCount,
		PresenceCount: &i.PresenceCount,
		Uses:          i.Uses,
		MaxUses:       i.MaxUses,
		MaxAgeSecs:    i.MaxAgeSecs,
		CreatedAt:     i.CreatedAt,
	}
}

func (i GuildInvite) Delete(ctx context.Context, rest *REST) error {
	return rest.DeleteInvite(ctx, i.Code)
}

func (i GuildInvite) DeleteWithReason(ctx context.Context, rest *REST, reason string) error {
	return rest.DeleteInviteWithReason(ctx, i.Code, reason)
}

type GroupDMInvite struct {
	Code        string        `json:"code"`
	Inviter     *User         `json:"inviter"`
	ExpiresAt   *time.Time    `json:"expires_at"`
	TempAccess  bool          `json:"temporary"`
	Channel     InviteChannel `json:"channel"`
	MemberCount int           `json:"member_count"`
}

func (i GroupDMInvite) ToInvite() Invite {
	return Invite{
		Code:        i.Code,
		Type:        InviteTypeGroupDM,
		Inviter:     i.Inviter,
		ExpiresAt:   i.ExpiresAt,
		TempAccess:  i.TempAccess,
		Channel:     &i.Channel,
		MemberCount: &i.MemberCount,
	}
}

func (i GroupDMInvite) Delete(ctx context.Context, rest *REST) error {
	return rest.DeleteInvite(ctx, i.Code)
}

func (i GroupDMInvite) DeleteWithReason(ctx context.Context, rest *REST, reason string) error {
	return rest.DeleteInviteWithReason(ctx, i.Code, reason)
}

// InviteGuild represents a guild that a [GuildInvite] leads to.
type InviteGuild struct {
	ID                  ID                       `json:"id"`
	Name                string                   `json:"name"`
	Icon                *string                  `json:"icon"`
	Banner              *string                  `json:"banner"`
	BannerWidth         *int                     `json:"banner_width"`
	BannerHeight        *int                     `json:"banner_height"`
	Splash              *string                  `json:"splash"`
	SplashWidth         *int                     `json:"splash_width"`
	SplashHeight        *int                     `json:"splash_height"`
	SplashCardAlignment GuildSplashCardAlignment `json:"splash_card_alignment"`
	EmbedSplash         *string                  `json:"embed_splash"`
	EmbedSplashWidth    *int                     `json:"embed_splash_width"`
	EmbedSplashHeight   *int                     `json:"embed_splash_height"`
	Features            []GuildFeature           `json:"features"`
}

// FullGuild returns the full guild object if it is cached.
func (g InviteGuild) FullGuild(cache *Cache) (Guild, bool) {
	return cache.Guilds.Get(g.ID)
}

// InviteChannel represents a channel that a [GuildInvite] or [GroupDMInvite] leads to.
type InviteChannel struct {
	ID         ID          `json:"id"`
	Name       *string     `json:"name"`
	Type       ChannelType `json:"type"`
	Recipients []User      `json:"recipients"`
}

// FullGuild returns the full channel object if it is cached.
func (c InviteChannel) FullChannel(cache *Cache) (Channel, bool) {
	return cache.Channel(c.ID)
}
