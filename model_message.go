package flo

import (
	"context"
	"time"
)

type Message struct {
	ID              ID
	ChannelID       ID
	Author          User
	WebhookID       *ID
	Type            MessageType
	Flags           MessageFlags
	Content         string
	EditedAt        *time.Time
	Pinned          bool
	MentionEveryone bool
	TTS             bool
	Mentions        []User
	MentionRoles    []ID
	Embeds          []Embed
	// TODO
	// Attachments     []struct{}
	// Stickers        []struct{}
	Reactions       []MessageReaction
	Nonce           *string
	Call            *MessageCall
}

func (m *Message) CreatedAt() time.Time {
	return m.ID.CreatedAt()
}

func (m *Message) IsDeletable() bool {
	return m.Type.IsDeletable()
}

func (m *Message) Delete(ctx context.Context, rest *REST) error {
	return rest.DeleteMessage(ctx, m.ChannelID, m.ID)
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
	EmbedChild
	Children []EmbedChild
}

type EmbedChild struct {
	Type        EmbedType
	URL         *string
	Title       *string
	Color       *ColorInt
	Timestamp   *time.Time
	Description *string
	Author      *EmbedAuthor
	Image       *EmbedMedia
	Thumbnail   *EmbedMedia
	Footer      *EmbedFooter
	Fields      []EmbedField
	Provider    *EmbedAuthor
	Video       *EmbedMedia
	Audio       *EmbedMedia
	NSFW        *bool
}

type EmbedType string

type EmbedAuthor struct {
	Name         string
	URL          *string
	IconURL      *string
	ProxyIconURL *string
}

type EmbedMedia struct {
	URL         string
	ProxyURL    *string
	ContentType *string
	ContentHash *string
	Width       *int
	Height      *int
	Description *string
	Placeholder *string
	Duration    *time.Duration
	Flags       EmbedMediaFlags
}

type EmbedMediaFlags uint

const (
	EmbedMediaFlagNSFW     = 1 << 4
	EmbedMediaFlagAnimated = 1 << 5
)

type EmbedFooter struct {
	Text         string
	IconURL      *string
	ProxyIconURL *string
}

type EmbedField struct {
	Name   string
	Value  string
	Inline bool
}

type ReactionEmoji struct {
	ID       *ID
	Name     string
	Animated bool
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

type MessageReaction struct {
	Emoji ReactionEmoji
	Count int
	Me    bool
}

type MessageCall struct {
	Participants []ID
	EndedAt      *time.Time
}
