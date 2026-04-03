package flo

import (
	"context"
	"time"
)

type Channel struct {
	ID               ID
	GuildID          ID
	Name             *string
	Topic            *string
	URL              *string
	Icon             *string
	OwnerID          *string
	Type             ChannelType
	Position         *int
	ParentID         *ID
	Bitrate          *int
	UserLimit        *int
	RTCRegion        *string
	LastMessageID    *ID
	LastPinAt        *time.Time
	PermOverwrites   []ChannelPermOverwrite
	Recipients       []any
	NSFW             *bool
	RateLimitPerUser *int
	Nicks            map[ID]string
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
	ID    ID
	Type  ChannelPermOverwriteType
	Allow Perms
	Deny  Perms
}
