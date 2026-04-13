package flo

import (
	"context"
	"fmt"
	"time"
)

//go:generate stringer -type=ChannelType,ChannelPermOverwriteType,MessageType,MessageReferenceType -output=model_channel_string.go

// Channel represents any kind of channel on Fluxer, which may or may not be able to hold messages.
type Channel struct {
	// ID is the globally unique identifier for the channel.
	ID ID `json:"id"`
	// Type is the type of channel.
	// Properties of which the presence can currently be used to determine the channel type are subject to change, so it is best to check this property if you want to guarantee the channel type.
	Type ChannelType `json:"type"`
	// GuildID is the guild this channel belongs to if this is a guild channel.
	GuildID *ID `json:"guild_id"`
	// Name is the name of the channel if applicable.
	// Regular DMs do not have this, and group DMs only have this if the name is overriden.
	Name *string `json:"name"`
	// Topic is the topic description of the channel if this is a guild channel which has it set.
	Topic *string `json:"topic"`
	// URL is the link the channel opens upon clicking it if the type is [ChannelTypeGuildLink].
	URL *string `json:"url"`
	// Icon is the icon hash of the channel if the type is [ChannelTypeGroupDM].
	Icon *string `json:"icon"`
	// OwnerID is the ID of the user owning this channel if the type is [ChannelTypeGroupDM].
	OwnerID *string `json:"owner_id"`
	// Position is used to sort guild channels.
	Position *int `json:"position"`
	// ParentID is the ID of the category this channel is in if this is a guild channel.
	ParentID *ID `json:"parent_id"`
	// Bitrate is the bitrate in bits per second if the type is [ChannelTypeGuildVoice].
	Bitrate *int `json:"bitrate"`
	// UserLimit is the maximum number of users allowed to join if the type is [ChannelTypeGuildVoice].
	UserLimit *int `json:"user_limit"`
	// RTCRegion is the voice region ID if the type is [ChannelTypeGuildVoice].
	RTCRegion *string `json:"rtc_region"`
	// LastMessageID is the ID of the last sent message if this is a textable channel.
	LastMessageID *ID `json:"last_message_id"`
	// LastPinAt is the time of the last pin if this is a textable channel.
	LastPinAt *time.Time `json:"last_pin_timestamp"`
	// PermOverwrites contains the permission overrides if this is a guild channel.
	PermOverwrites []ChannelPermOverwrite `json:"permission_overwrites"`
	// Recipients contains the users with access to the channel if the type is [ChannelTypeDM] or [ChannelTypeGroupDM].
	Recipients []User `json:"recipients"`
	// NSFW is true if the channel is marked as age-restricted.
	NSFW bool `json:"nsfw"`
	// RateLimitSecs is the slowmode duration in seconds.
	RateLimitSecs int `json:"rate_limit_per_user"`
	// Nicks contains custom nicknames for users inside the channel if the type is [ChannelTypeGroupDM].
	Nicks map[ID]string `json:"nicks"`

	Messages *Collection[Message]
}

func (c *Channel) CreatedAt() time.Time {
	return c.ID.CreatedAt()
}

// Mention creates a string which can be used to display the channel in chat.
func (c *Channel) Mention() string {
	return fmt.Sprintf("<#%d>", c.ID)
}

// IsTextable returns true if the channel can contains messages.
func (c *Channel) IsTextable() bool {
	return c.Type.IsTextable()
}

func (c *Channel) CreateMessage(rest *REST, ctx context.Context, opts CreateMessageOpts) (Message, error) {
	return rest.CreateMessage(ctx, c.ID, opts)
}

type ChannelType uint

const (
	ChannelTypeGuildText     ChannelType = 0
	ChannelTypeDM            ChannelType = 1
	ChannelTypeGuildVoice    ChannelType = 2
	ChannelTypeGroupDM       ChannelType = 3
	ChannelTypeGuildCategory ChannelType = 4
	ChannelTypeGuildLink     ChannelType = 998
	ChannelTypePersonalNotes ChannelType = 999
)

var textableChannelTypes = [...]bool{
	ChannelTypeGuildText:     true,
	ChannelTypeDM:            true,
	ChannelTypeGuildVoice:    false,
	ChannelTypeGroupDM:       true,
	ChannelTypeGuildCategory: false,
	ChannelTypeGuildLink:     false,
	ChannelTypePersonalNotes: true,
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
	// ID is the globally unique identifier for the message.
	ID ID `json:"id"`
	// ChannelID is the channel that the message belongs to.
	ChannelID ID `json:"channel_id"`
	// Type is the type of message.
	Type MessageType `json:"type"`
	// Author is the user which sent the message.
	// If WebhookID is not nil, this is a fake webhook user.
	// For non-default/reply messages this is the user performing the action.
	Author User `json:"author"`
	// WebhookID is the ID of the webhook which sent the message.
	WebhookID *ID `json:"webhook_id"`
	// Flags is a set of flags on the message.
	Flags MessageFlags `json:"flags"`
	// Content is the textual content of the message, if any.
	// The meaning varies widely depending on the message type.
	// You should probably check the message type before reading this.
	Content string `json:"content"`
	// EditedAt is the time when the message was last edited.
	EditedAt *time.Time `json:"edited_timestamp"`
	// Pinned is true if the message is pinned.
	Pinned bool `json:"pinned"`
	// MentionEveryone is true if the message mentions @everyone.
	MentionEveryone bool `json:"mention_everyone"`
	// TTS is true if the message is text-to-speech.
	TTS bool `json:"tts"`
	// Mentions contains the users mentioned by the message.
	Mentions []User `json:"mentions"`
	// MentionRoles contains the roles mentioned by the message.
	MentionRoles []ID `json:"mention_roles"`
	// Embeds contains the embeds attached to the message.
	Embeds []Embed `json:"embeds"`
	// Attachments contains the files attached to the attached
	Attachments []Attachment `json:"attachments"`
	// Stickers contains the stickers sent with the message.
	Stickers []MessageSticker `json:"stickers"`
	// Reactions contains the reactions on the message.
	Reactions []MessageReaction `json:"reactions"`
	// MessageReference identifies the forwarded or replied to message.
	MessageReference *MessageReference `json:"message_reference"`
	// ReferencedMessage is the message that is being replied to.
	ReferencedMessage *Message `json:"referenced_message"` // TODO: also add MessageSnaphots
	// Call specifies the call the message represents if the type is [MessageTypeCall].
	Call *MessageCall `json:"call"`
}

func (m *Message) CreatedAt() time.Time {
	return m.ID.CreatedAt()
}

func (m *Message) IsDeletable() bool {
	return m.Type.IsDeletable()
}

func (m *Message) Edit(rest *REST, ctx context.Context, opts EditMessageOpts) error {
	msg, err := rest.EditMessage(ctx, m.ChannelID, m.ID, opts)
	if err != nil {
		return err
	}

	*m = msg
	return nil
}

func (m *Message) Delete(rest *REST, ctx context.Context) error {
	return rest.DeleteMessage(ctx, m.ChannelID, m.ID)
}

func (m *Message) updateCache(cache *Cache) {
	if m.WebhookID == nil {
		cache.Users.Set(m.Author.ID, m.Author)
	}

	for _, user := range m.Mentions {
		cache.Users.Set(user.ID, user)
	}

	if m.ReferencedMessage != nil {
		referenced := *m.ReferencedMessage
		// NOTE: prevent recursion, just in case
		referenced.ReferencedMessage = nil
		referenced.updateCache(cache)
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

// Render creates a string that can be used to display the emoji in chat.
func (e *ReactionEmoji) Render() string {
	if unicode, ok := e.UnicodeEmoji(); ok {
		return unicode
	} else if !e.Animated {
		return fmt.Sprintf("<:%s:%d>", e.Name, *e.ID)
	} else {
		return fmt.Sprintf("<a:%s:%d>", e.Name, *e.ID)

	}
}

func (e *ReactionEmoji) IsUnicode() bool {
	return e.ID == nil
}

func (e *ReactionEmoji) IsCustom() bool {
	return e.ID != nil
}

// UnicodeEmoji returns the emoji string if a unicode emoji was reacted with.
// Otherwise, the returned bool will be false.
func (e *ReactionEmoji) UnicodeEmoji() (string, bool) {
	if e.ID == nil {
		return e.Name, true
	} else {
		return "", false
	}
}

// CustomEmoji returns the custom emoji ID and name if a custom (non-unicode) emoji was reacted with.
// Otherwise, the returned bool will be false.
func (e *ReactionEmoji) CustomEmoji() (ID, string, bool) {
	if e.ID != nil {
		return *e.ID, e.Name, true
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
