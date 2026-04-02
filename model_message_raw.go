package flo

import "time"

type rawMessage struct {
	ID              ID           `json:"id"`
	ChannelID       ID           `json:"channel_id"`
	Author          User         `json:"author"`
	WebhookID       *ID          `json:"webhook_id"`
	Type            MessageType  `json:"type"`
	Flags           MessageFlags `json:"flags"`
	Content         string       `json:"content"`
	EditedTimestamp *time.Time   `json:"edited_timestamp"`
	Pinned          bool         `json:"pinned"`
	MentionEveryone bool         `json:"mention_everyone"`
	TTS             bool         `json:"tts"`
	Mentions        []User       `json:"mentions"`
	MentionRoles    []ID         `json:"mention_roles"`
	Embeds          []Embed      `json:"embeds"`
	// TODO
	// Attachments     []struct{}
	// Stickers        []struct{}
	Reactions []MessageReaction `json:"reactions"`
	Nonce     *string           `json:"nonce"`
	Call      *MessageCall      `json:"call"`
}
