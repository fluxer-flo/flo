package flo

import (
	"context"
	"time"
)

//go:generate stringer -type=ChannelType,ChannelPermOverwriteType -output=model_channel_string.go


type Channel struct {
	ID               ID                     `json:"id"`
	GuildID          ID                     `json:"guild_id"`
	Name             *string                `json:"name"`
	Topic            *string                `json:"topic"`
	URL              *string                `json:"url"`
	Icon             *string                `json:"icon"`
	OwnerID          *string                `json:"owner_id"`
	Type             ChannelType            `json:"type"`
	Position         *int                   `json:"position"`
	ParentID         *ID                    `json:"parent_id"`
	Bitrate          *int                   `json:"bitrate"`
	UserLimit        *int                   `json:"user_limit"`
	RTCRegion        *string                `json:"rtc_region"`
	LastMessageID    *ID                    `json:"last_message_id"`
	LastPinAt        *time.Time             `json:"last_pin_timestamp"`
	PermOverwrites   []ChannelPermOverwrite `json:"permission_overwrites"`
	Recipients       []any                  `json:"recipients"`
	NSFW             *bool                  `json:"nsfw"`
	RateLimitPerUser *int                   `json:"rate_limit_per_user"`
	Nicks            map[ID]string          `json:"nicks"`
}

func (c *Channel) CreatedAt() time.Time {
	return c.ID.CreatedAt()
}

func (c *Channel) IsTextable() bool {
	return c.Type.IsTextable()
}

func (c *Channel) SendMessage(rest *REST, ctx context.Context, opts SendMessageOpts) (Message, error) {
	return rest.SendMessage(ctx, c.ID, opts)
}

func (c *Channel) SendMessageContent(rest *REST, ctx context.Context, content string) (Message, error) {
	return rest.SendMessageContent(ctx, c.ID, content)
}

type ChannelType uint

const (
	ChannelTypeGuildText       ChannelType = 0
	ChannelTypeDM              ChannelType = 1
	ChannelTypeGuildVoice      ChannelType = 2
	ChannelTypeGroupDM         ChannelType = 3
	ChannelTypeGuildCategory   ChannelType = 4
	ChannelTypeGuildLink       ChannelType = 998
	ChannelTypeDMPersonalNotes ChannelType = 999
)

var textableChannelTypes = [...]bool{
	ChannelTypeGuildText:       true,
	ChannelTypeDM:              true,
	ChannelTypeGuildVoice:      false,
	ChannelTypeGroupDM:         true,
	ChannelTypeGuildCategory:   false,
	ChannelTypeGuildLink:       false,
	ChannelTypeDMPersonalNotes: true,
}

func (c ChannelType) IsTextable() bool {
	return c < ChannelType(len(textableChannelTypes)) && textableChannelTypes[c]
}

type ChannelPermOverwriteType uint

const (
	ChannelPermOverwriteTypeRole   ChannelPermOverwriteType = 0
	ChannelPermOverwriteTypeMember ChannelPermOverwriteType = 1
)

type ChannelPermOverwrite struct {
	ID    ID                       `json:"id"`
	Type  ChannelPermOverwriteType `json:"type"`
	Allow Perms                    `json:"allow"`
	Deny  Perms                    `json:"deny"`
}
