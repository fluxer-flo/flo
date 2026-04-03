package flo

import "time"

type rawMessage struct {
	ID              ID                   `json:"id"`
	ChannelID       ID                   `json:"channel_id"`
	Author          rawUser              `json:"author"`
	WebhookID       *ID                  `json:"webhook_id"`
	Type            MessageType          `json:"type"`
	Flags           MessageFlags         `json:"flags"`
	Content         string               `json:"content"`
	EditedAt        *time.Time           `json:"edited_timestamp"`
	Pinned          bool                 `json:"pinned"`
	MentionEveryone bool                 `json:"mention_everyone"`
	TTS             bool                 `json:"tts"`
	Mentions        []rawUser            `json:"mentions"`
	MentionRoles    []ID                 `json:"mention_roles"`
	Embeds          []rawEmbed           `json:"embeds"`
	Attachments     []rawAttachment      `json:"attachments"`
	Stickers        []rawMessageSticker  `json:"stickers"`
	Reactions       []rawMessageReaction `json:"reactions"`
	Nonce           *string              `json:"nonce"`
	Call            *rawMessageCall      `json:"call"`
}

type rawEmbed struct {
	Type        EmbedType       `json:"type"`
	URL         *string         `json:"url"`
	Title       *string         `json:"title"`
	Color       *ColorInt       `json:"color"`
	Timestamp   *time.Time      `json:"timestamp"`
	Description *string         `json:"description"`
	Author      *rawEmbedAuthor `json:"author"`
	Image       *rawEmbedMedia  `json:"image"`
	Thumbnail   *rawEmbedMedia  `json:"thumbnail"`
	Footer      *rawEmbedFooter `json:"footer"`
	Fields      []rawEmbedField `json:"fields"`
	Provider    *rawEmbedAuthor `json:"provider"`
	Video       *rawEmbedMedia  `json:"video"`
	Audio       *rawEmbedMedia  `json:"audio"`
	NSFW        *bool           `json:"nsfw"`
	Children    []rawEmbed      `json:"children"`
}

type rawEmbedAuthor struct {
	Name         string  `json:"name"`
	URL          *string `json:"url"`
	IconURL      *string `json:"icon_url"`
	ProxyIconURL *string `json:"proxy_icon_url"`
}

type rawEmbedMedia struct {
	URL         string          `json:"url"`
	ProxyURL    *string         `json:"proxy_url"`
	ContentType *string         `json:"content_type"`
	ContentHash *string         `json:"content_hash"`
	Width       *int            `json:"width"`
	Height      *int            `json:"height"`
	Description *string         `json:"description"`
	Placeholder *string         `json:"placeholder"`
	Duration    *int            `json:"duration"`
	Flags       EmbedMediaFlags `json:"flags"`
}

type rawEmbedFooter struct {
	Text         string  `json:"text"`
	IconURL      *string `json:"icon_url"`
	ProxyIconURL *string `json:"proxy_icon_url"`
}

type rawEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type rawMessageSticker struct {
	ID       ID     `json:"id"`
	Name     string `json:"name"`
	Animated bool   `json:"animated"`
}

type rawAttachment struct {
	ID          ID              `json:"id"`
	Filename    string          `json:"filename"`
	Title       *string         `json:"title"`
	Description *string         `json:"description"`
	ContentType *string         `json:"content_type"`
	ContentHash *string         `json:"content_hash"`
	Size        int             `json:"size"`
	URL         *string         `json:"url"`
	ProxyURL    *string         `json:"proxy_url"`
	Width       *int            `json:"width"`
	Height      *int            `json:"height"`
	Placeholder *string         `json:"placeholder"`
	Flags       AttachmentFlags `json:"flags"`
	Duration    *int            `json:"duration"`
	Waveform    *string         `json:"waveform"`
	ExpiresAt   *time.Time      `json:"expires_at"`
	Expired     bool            `json:"expired"`
}

type rawMessageReaction struct {
	Emoji rawReactionEmoji `json:"emoji"`
	Count int              `json:"count"`
	Me    bool             `json:"me"`
}

type rawReactionEmoji struct {
	ID       *ID    `json:"id"`
	Name     string `json:"name"`
	Animated bool   `json:"animated"`
}

type rawMessageCall struct {
	Participants []ID       `json:"participants"`
	EndedAt      *time.Time `json:"ended_timestamp"`
}

func (m *Message) update(cache Cache, msg *rawMessage) {
	m.ChannelID = msg.ChannelID
	m.Author = cacheUser(cache, &msg.Author)
	m.WebhookID = msg.WebhookID
	m.Type = msg.Type
	m.Flags = msg.Flags
	m.Content = msg.Content
	m.EditedAt = msg.EditedAt
	m.Pinned = msg.Pinned
	m.MentionEveryone = msg.MentionEveryone
	m.TTS = msg.TTS

	if msg.Mentions != nil {
		m.Mentions = make([]User, 0, len(msg.Mentions))
		for _, user := range msg.Mentions {
			m.Mentions = append(m.Mentions, cacheUser(cache, &user))
		}
	}

	m.MentionRoles = msg.MentionRoles

	convEmbedMedia := func(media *rawEmbedMedia) *EmbedMedia {
		if media == nil {
			return nil
		}

		result := EmbedMedia{
			URL:         media.URL,
			ProxyURL:    media.ProxyURL,
			ContentType: media.ContentType,
			ContentHash: media.ContentHash,
			Width:       media.Width,
			Height:      media.Height,
			Description: media.Description,
			Placeholder: media.Placeholder,
			Flags:       media.Flags,
		}

		if media.Duration != nil {
			d := time.Duration(*media.Duration) * time.Second
			result.Duration = &d
		}

		return &result
	}

	convEmbed := func(embed rawEmbed) Embed {
		result := Embed{
			Type:        embed.Type,
			URL:         embed.URL,
			Title:       embed.Title,
			Color:       embed.Color,
			Timestamp:   embed.Timestamp,
			Description: embed.Description,
			Author:      (*EmbedAuthor)(embed.Author),
			Image:       convEmbedMedia(embed.Image),
			Thumbnail:   convEmbedMedia(embed.Thumbnail),
			Footer:      (*EmbedFooter)(embed.Footer),
			Provider:    (*EmbedAuthor)(embed.Provider),
			Video:       convEmbedMedia(embed.Video),
			Audio:       convEmbedMedia(embed.Audio),
		}

		if embed.Fields != nil {
			// it ain't pretty, but it has to be done:
			// https://go.dev/doc/faq#convert_slice_with_same_underlying_type
			result.Fields = make([]EmbedField, 0, len(embed.Fields))
			for _, field := range embed.Fields {
				result.Fields = append(result.Fields, EmbedField(field))
			}
		}

		return result
	}

	if msg.Embeds != nil {
		m.Embeds = make([]Embed, 0, len(msg.Embeds))
		for _, rawEmbed := range msg.Embeds {
			embed := convEmbed(rawEmbed)

			if rawEmbed.Children != nil {
				embed.Children = make([]Embed, 0, len(rawEmbed.Children))
				for _, child := range rawEmbed.Children {
					embed.Children = append(embed.Children, convEmbed(child))
				}
			}

			m.Embeds = append(m.Embeds, embed)
		}
	}

	if msg.Attachments != nil {
		m.Attachments = make([]Attachment, 0, len(msg.Attachments))
		for _, rawAttachment := range msg.Attachments {
			attachment := Attachment{
				ID:          rawAttachment.ID,
				Filename:    rawAttachment.Filename,
				Title:       rawAttachment.Title,
				Description: rawAttachment.Description,
				ContentType: rawAttachment.ContentType,
				ContentHash: rawAttachment.ContentHash,
				Size:        rawAttachment.Size,
				ProxyURL:    rawAttachment.ProxyURL,
				Width:       rawAttachment.Width,
				Height:      rawAttachment.Height,
				Placeholder: rawAttachment.Placeholder,
				Flags:       rawAttachment.Flags,
				Waveform:    rawAttachment.Waveform,
				ExpiresAt:   rawAttachment.ExpiresAt,
				Expired:     rawAttachment.Expired,
			}

			if rawAttachment.Duration != nil {
				d := time.Duration(*rawAttachment.Duration) * time.Second
				attachment.Duration = &d
			}

			m.Attachments = append(m.Attachments, attachment)
		}
	}

	if msg.Reactions != nil {
		m.Reactions = make([]MessageReaction, 0, len(msg.Reactions))
		for _, reaction := range msg.Reactions {
			m.Reactions = append(m.Reactions, MessageReaction{
				Emoji: ReactionEmoji(reaction.Emoji),
				Count: reaction.Count,
				Me:    reaction.Me,
			})
		}
	}

	m.Nonce = msg.Nonce
	m.Call = (*MessageCall)(msg.Call)
}
