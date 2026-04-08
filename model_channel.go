package flo

import (
	"context"
	"time"
)

//go:generate stringer -type=ChannelType,ChannelPermOverwriteType,MessageType,MessageReferenceType -output=model_channel_string.go

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

func (c *Channel) CreateMessage(rest *REST, ctx context.Context, opts CreateMessageOpts) (Message, error) {
	return rest.CreateMessage(ctx, c.ID, opts)
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

type Message struct {
	ID               ID                `json:"id"`
	ChannelID        ID                `json:"channel_id"`
	Author           User              `json:"author"`
	WebhookID        *ID               `json:"webhook_id"`
	Type             MessageType       `json:"type"`
	Flags            MessageFlags      `json:"flags"`
	Content          string            `json:"content"`
	EditedAt         *time.Time        `json:"edited_timestamp"`
	Pinned           bool              `json:"pinned"`
	MentionEveryone  bool              `json:"mention_everyone"`
	TTS              bool              `json:"tts"`
	Mentions         []User            `json:"mentions"`
	MentionRoles     []ID              `json:"mention_roles"`
	Embeds           []Embed           `json:"embeds"`
	Attachments      []Attachment      `json:"attachments"`
	Stickers         []MessageSticker  `json:"stickers"`
	Reactions        []MessageReaction `json:"reactions"`
	MessageReference MessageReference  `json:"message_reference"`
	Nonce            *string           `json:"nonce"`
	Call             *MessageCall      `json:"call"`
}

func (m *Message) CreatedAt() time.Time {
	return m.ID.CreatedAt()
}

func (m *Message) IsDeletable() bool {
	return m.Type.IsDeletable()
}

func (m *Message) Delete(rest *REST, ctx context.Context) error {
	return rest.DeleteMessage(ctx, m.ChannelID, m.ID)
}

func (m *Message) updateCache(cache *Cache) {
	cache.Users.Set(m.Author.ID, m.Author)

	for _, user := range m.Mentions {
		cache.Users.Set(user.ID, user)
	}
}

type MessageType uint

const (
	MessageTypeDefault              MessageType = 0
	MessageTypeRecipientAdd         MessageType = 1
	MessageTypeRecipientRemove      MessageType = 2
	MessageTypeCall                 MessageType = 3
	MessageTypeChannelNameChange    MessageType = 4
	MessageTypeChannelIconChange    MessageType = 5
	MessageTypeChannelPinnedMessage MessageType = 6
	MessageTypeUserJoin             MessageType = 7
	MessageTypeReply                MessageType = 19
)

var deletableMessageTypes = [...]bool{
	MessageTypeDefault:              true,
	MessageTypeReply:                true,
	MessageTypeChannelPinnedMessage: true,
	MessageTypeUserJoin:             true,
	MessageTypeRecipientAdd:         false,
	MessageTypeRecipientRemove:      false,
	MessageTypeCall:                 false,
	MessageTypeChannelNameChange:    false,
	MessageTypeChannelIconChange:    false,
}

func (mt MessageType) IsDeletable() bool {
	return mt < MessageType(len(deletableMessageTypes)) && deletableMessageTypes[mt]
}

type MessageFlags uint

const (
	MessageFlagSupressEmbeds       MessageFlags = 1 << 2
	MessageFlagSupressNotification MessageFlags = 1 << 12
	MessageFlagCompactAttachments  MessageFlags = 1 << 17
)

type Embed struct {
	Type        EmbedType    `json:"type"`
	URL         *string      `json:"url"`
	Title       *string      `json:"title"`
	Color       *ColorInt    `json:"color"`
	Timestamp   *time.Time   `json:"timestamp"`
	Description *string      `json:"description"`
	Author      *EmbedAuthor `json:"author"`
	Image       *EmbedMedia  `json:"image"`
	Thumbnail   *EmbedMedia  `json:"thumbnail"`
	Footer      *EmbedFooter `json:"footer"`
	Fields      []EmbedField `json:"fields"`
	Provider    *EmbedAuthor `json:"provider"`
	Video       *EmbedMedia  `json:"video"`
	Audio       *EmbedMedia  `json:"audio"`
	// Children is a list of internal nested embeds generated by unfurlers.
	// Each child will not contain any further children.
	Children []Embed `json:"children"`
}

type EmbedType string

type EmbedAuthor struct {
	Name         string  `json:"name"`
	URL          *string `json:"url"`
	IconURL      *string `json:"icon_url"`
	ProxyIconURL *string `json:"proxy_icon_url"`
}

type EmbedMedia struct {
	URL         string          `json:"url"`
	ProxyURL    *string         `json:"proxy_url"`
	ContentType *string         `json:"content_type"`
	ContentHash *string         `json:"content_hash"`
	Width       *int            `json:"width"`
	Height      *int            `json:"height"`
	Description *string         `json:"description"`
	Placeholder *string         `json:"placeholder"`
	Duration    *time.Duration  `json:"duration"`
	Flags       EmbedMediaFlags `json:"flags"`
}

type EmbedMediaFlags uint

const (
	EmbedMediaFlagNSFW     = 1 << 4
	EmbedMediaFlagAnimated = 1 << 5
)

type EmbedFooter struct {
	Text         string  `json:"text"`
	IconURL      *string `json:"icon_url"`
	ProxyIconURL *string `json:"proxy_icon_url"`
}

type EmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type Attachment struct {
	ID           ID              `json:"id"`
	Filename     string          `json:"filename"`
	Title        *string         `json:"title"`
	Description  *string         `json:"description"`
	ContentType  *string         `json:"content_type"`
	ContentHash  *string         `json:"content_hash"`
	Size         int             `json:"size"`
	URL          *string         `json:"url"`
	ProxyURL     *string         `json:"proxy_url"`
	Width        *int            `json:"width"`
	Height       *int            `json:"height"`
	Placeholder  *string         `json:"placeholder"`
	Flags        AttachmentFlags `json:"flags"`
	DurationSecs *int            `json:"duration"`
	Waveform     *string         `json:"waveform"`
	ExpiresAt    *time.Time      `json:"expires_at"`
	Expired      bool            `json:"expired"`
}

type AttachmentFlags uint

const (
	AttachmentFlagSpoiler  = 1 << 3
	AttachmentFlagNSFW     = 1 << 4
	AttachmentFlagAnimated = 1 << 5
)

type MessageSticker struct {
	ID       ID     `json:"id"`
	Name     string `json:"name"`
	Animated bool   `json:"animated"`
}

type MessageReaction struct {
	Emoji ReactionEmoji `json:"emoji"`
	Count int           `json:"count"`
	Me    bool          `json:"me"`
}

type ReactionEmoji struct {
	ID       *ID    `json:"id"`
	Name     string `json:"name"`
	Animated bool   `json:"animated"`
}

func (re *ReactionEmoji) IsUnicode() bool {
	return re.ID == nil
}

func (re *ReactionEmoji) IsCustom() bool {
	return re.ID != nil
}

// UnicodeEmoji returns the emoji string if a unicode emoji was reacted with.
// Otherwise, the returned bool will be false.
func (re *ReactionEmoji) UnicodeEmoji() (string, bool) {
	if re.ID == nil {
		return re.Name, true
	} else {
		return "", false
	}
}

// CustomEmoji returns the custom emoji ID and name if a custom (non-unicode) emoji was reacted with.
// Otherwise, the returned bool will be false.
func (re *ReactionEmoji) CustomEmoji() (ID, string, bool) {
	if re.ID != nil {
		return *re.ID, re.Name, true
	} else {
		return 0, "", false
	}
}

type MessageReference struct {
	ChannelID ID                   `json:"channel_id"`
	MessageID ID                   `json:"message_id"`
	GuildID   *ID                  `json:"guild_id"`
	Type      MessageReferenceType `json:"type"`
}

type MessageReferenceType uint

const (
	MessageReferenceTypeDefault MessageReferenceType = 0
	MessageReferenceTypeForward MessageReferenceType = 1
)

type MessageCall struct {
	Participants []ID       `json:"participants"`
	EndedAt      *time.Time `json:"ended_timestamp"`
}
