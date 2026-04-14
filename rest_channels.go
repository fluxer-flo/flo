package flo

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"
)

func rateLimitReadChannel(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:read:%d", channelID),
		Limit:  100,
		Window: 10 * time.Second,
	}
}

func (r *REST) GetChannel(ctx context.Context, channelID ID) (Channel, error) {
	var resp Channel
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "GET",
		Path:      fmt.Sprintf("/v1/channels/%d", channelID),
		RateLimit: rateLimitReadChannel(channelID),
	}, &resp)
	if err != nil {
		return Channel{}, err
	}

	cacheChannel(&resp, r.Cache)
	return resp, nil
}

type UpdateChannelOpts struct {
	Name           *string                `json:"name,omitempty"`
	Topic          *string                `json:"topic,omitempty"`
	URL            *string                `json:"url,omitempty"`
	Icon           *string                `json:"icon,omitempty"`
	OwnerID        *string                `json:"owner_id,omitempty"`
	Position       *int                   `json:"position,omitempty"`
	ParentID       *ID                    `json:"parent_id,omitempty"`
	Bitrate        *int                   `json:"bitrate,omitempty"`
	UserLimit      *int                   `json:"user_limit,omitempty"`
	RTCRegion      *string                `json:"rtc_region,omitempty"`
	PermOverwrites []ChannelPermOverwrite `json:"permission_overwrites,omitzero"`
	Recipients     []User                 `json:"recipients,omitzero"`
	NSFW           *bool                  `json:"nsfw,omitempty"`
	RateLimitSecs  *int                   `json:"rate_limit_per_user,omitempty"`
	Nicks          map[ID]string          `json:"nicks,omitzero"`
}

func rateLimitUpdateChannel(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:update:%d", channelID),
		Limit:  20,
		Window: 10 * time.Second,
	}
}

func (r *REST) UpdateChannel(ctx context.Context, channelID ID, opts UpdateChannelOpts) (Channel, error) {
	var resp Channel
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "PATCH",
		Path:      fmt.Sprintf("/v1/channels/%d", channelID),
		RateLimit: rateLimitUpdateChannel(channelID),
		Payload:   opts,
	}, &resp)
	if err != nil {
		return Channel{}, err
	}

	cacheChannel(&resp, r.Cache)
	return resp, nil
}

func rateLimitDeleteChannel(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:delete:%d", channelID),
		Limit:  20,
		Window: 10 * time.Second,
	}
}

func (r *REST) DeleteChannel(ctx context.Context, channelID ID) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method:    "DELETE",
		Path:      fmt.Sprintf("/v1/channels/%d", channelID),
		RateLimit: rateLimitDeleteChannel(channelID),
	})
}

type GetMessagesOpts struct {
	// Around is specified as around=... in the URL if not 0.
	Around ID
	// Before is specified as before=... in the URL if not 0.
	Before ID
	// After is specified as after=... in the URL if not 0.
	After ID
	// Limit is specified as limit=... in the URL if not 0.
	Limit uint
}

func rateLimitReadMessages(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:messages:read:%d", channelID),
		Limit:  100,
		Window: 10 * time.Second,
	}
}

func (r *REST) GetMessages(ctx context.Context, channelID ID, opts GetMessagesOpts) ([]Message, error) {
	query := url.Values{}
	if opts.Around != 0 {
		query.Set("around", fmt.Sprint(opts.Around))
	}
	if opts.Before != 0 {
		query.Set("before", fmt.Sprint(opts.Before))
	}
	if opts.After != 0 {
		query.Set("after", fmt.Sprint(opts.After))
	}
	if opts.Limit != 0 {
		query.Set("limit", fmt.Sprint(opts.Limit))
	}

	var resp []Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "GET",
		Path:      fmt.Sprintf("/v1/channels/%d/messages", channelID),
		Query:     query.Encode(),
		RateLimit: rateLimitReadMessages(channelID),
	}, &resp)
	if err != nil {
		return nil, err
	}

	cacheMessages(channelID, resp, r.Cache)
	return resp, nil
}

func rateLimitReadMessage(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:message:read:%d", channelID),
		Limit:  100,
		Window: 10 * time.Second,
	}
}

func (r *REST) GetMessage(ctx context.Context, channelID ID, msgID ID) (Message, error) {
	var resp Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "GET",
		Path:      fmt.Sprintf("/v1/channels/%d/messages/%d", channelID, msgID),
		RateLimit: rateLimitReadMessage(channelID),
	}, &resp)
	if err != nil {
		return Message{}, err
	}

	cacheMessage(&resp, r.Cache)
	return resp, nil
}

// CreateMessageOpts specifies a message to send.
type CreateMessageOpts struct {
	Content          string                 `json:"content,omitempty"`
	Embeds           []EmbedOpts            `json:"embeds,omitempty"`
	Attachments      []CreateAttachmentOpts `json:"attachments,omitempty"`
	MessageReference MessageReferenceOpts   `json:"message_reference,omitzero"`
	AllowedMentions  *AllowedMentions       `json:"allowed_mentions,omitempty"`
	Flags            MessageFlags           `json:"flags,omitzero"`
	Nonce            string                 `json:"nonce,omitempty"`
	StickerIDs       []ID                   `json:"sticker_ids,omitempty"`
	TTS              bool                   `json:"tts,omitzero"`
}

// EmbedOpts specifies a rich embed when creating or editing a message.
type EmbedOpts struct {
	URL         string          `json:"url,omitempty"`
	Title       string          `json:"title,omitempty"`
	Color       ColorInt        `json:"color,omitzero"`
	Timestamp   time.Time       `json:"timestamp,omitzero"`
	Description string          `json:"description,omitempty"`
	Author      EmbedAuthorOpts `json:"author,omitzero"`
	Image       EmbedMediaOpts  `json:"image,omitzero"`
	Thumbnail   EmbedMediaOpts  `json:"thumbnail,omitzero"`
	Footer      EmbedFooterOpts `json:"footer,omitzero"`
	Fields      []EmbedField    `json:"fields,omitempty"`
}

// EmbedAuthorOpts specifies an embed author when creating or editing a message.
type EmbedAuthorOpts struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// EmbedAuthorOpts specifies embed media when creating or editing a message.
type EmbedMediaOpts struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// EmbedAuthorOpts specifies an embed footer when creating or editing a message.
type EmbedFooterOpts struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// CreateAttachmentOpts specifies an attachment when creating a message.
type CreateAttachmentOpts struct {
	// ID is the fake ID used for the attachment in the request.
	// If left as 0, ascending numbers will be used.
	// An actual [ID] will be generated in the response.
	ID          uint            `json:"id"`
	Filename    string          `json:"filename"`
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Flags       AttachmentFlags `json:"flags,omitzero"`
	// Content is used to provide attachment data. It should not be nil.
	Content io.ReadCloser `json:"-"`
	// TODO: waveform?
}

// AllowedMentions specifies allowed mentions when creating or editing a message.
type AllowedMentions struct {
	Parse       []AllowedMentionsParse `json:"parse,omitzero"`
	Users       []ID                   `json:"users,omitzero"`
	Roles       []ID                   `json:"roles,omitzero"`
	RepliedUser *bool                  `json:"replied_user,omitempty"`
}

type AllowedMentionsParse string

const (
	AllowedMentionsParseUsers    AllowedMentionsParse = "users"
	AllowedMentionsParseRoles    AllowedMentionsParse = "roles"
	AllowedMentionsParseEveryone AllowedMentionsParse = "everyone"
)

// MessageReferenceOpts specifies a message reference (reply or forward) when sending or editing a message.
type MessageReferenceOpts struct {
	MessageID ID                   `json:"message_id"`
	ChannelID ID                   `json:"channel_id,omitzero"`
	GuildID   ID                   `json:"guild_id,omitzero"`
	Type      MessageReferenceType `json:"type"`
}

func rateLimitCreateMessage(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:message:create:%d", channelID),
		Limit:  20,
		Window: 10 * time.Second,
	}
}

func (r *REST) CreateMessage(ctx context.Context, channelID ID, opts CreateMessageOpts) (Message, error) {
	if opts.AllowedMentions == nil {
		opts.AllowedMentions = r.DefaultAllowedMentions
	}

	var files []RESTFormField
	if len(opts.Attachments) != 0 {
		files = make([]RESTFormField, 0, len(opts.Attachments))

		var id uint
		for i := range opts.Attachments {
			attachment := &opts.Attachments[i]
			if attachment.ID == 0 {
				id++
				attachment.ID = id
			}

			files = append(files, RESTFormField{
				FieldName: fmt.Sprintf("files[%d]", id),
				FileName:  "-",
				Content:   attachment.Content,
			})
		}
	}

	var resp Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "POST",
		Path:      fmt.Sprintf("/v1/channels/%d/messages", channelID),
		RateLimit: rateLimitCreateMessage(channelID),
		Payload:   opts,
		Form:      files,
	}, &resp)
	if err != nil {
		return Message{}, err
	}

	cacheMessage(&resp, r.Cache)
	return resp, nil
}

// EditMessageOpts specifies message fields to edit.
// A field being left as nil indicates to keep it the same.
type EditMessageOpts struct {
	Content         *string              `json:"content,omitempty"`
	Embeds          []Embed              `json:"embeds,omitzero"`
	AllowedMentions *AllowedMentions     `json:"allowed_mentions,omitempty"`
	Attachments     []EditAttachmentOpts `json:"attachments,omitzero"`
	Flags           *MessageFlags        `json:"flags,omitempty"`
}

// EditAttachmentOpts specifies a possibly preexisting attachment when editing a message.
type EditAttachmentOpts struct {
	// ID is the ID of an existing attachment to keep or a fake ID for a new attachment.
	// If left as 0 ascending numbers will be be used in the request - you probably want to do this for new attachments.
	// An actual [ID] will be generated for new attachments in the response.
	ID       ID     `json:"id"`
	Filename string `json:"filename"`
	// Content is used to provide attachment data. It should be nil for existing attachments but not for new ones.
	Content io.ReadCloser `json:"-"`
}

func (r *REST) EditMessage(ctx context.Context, channelID ID, msgID ID, opts EditMessageOpts) (Message, error) {
	if opts.AllowedMentions == nil {
		opts.AllowedMentions = r.DefaultAllowedMentions
	}

	var files []RESTFormField
	if len(opts.Attachments) != 0 {
		files = make([]RESTFormField, 0, len(opts.Attachments))

		var id ID
		for i := range opts.Attachments {
			attachment := &opts.Attachments[i]
			if attachment.ID == 0 {
				id++
				attachment.ID = id
			}

			if attachment.Content != nil {
				files = append(files, RESTFormField{
					FieldName: fmt.Sprintf("files[%d]", id),
					FileName:  "-",
					Content:   attachment.Content,
				})
			}
		}
	}

	var resp Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "PATCH",
		Path:      fmt.Sprintf("/v1/channels/%d/messages/%d", channelID, msgID),
		RateLimit: rateLimitCreateMessage(channelID),
		Payload:   opts,
		Form:      files,
	}, &resp)
	if err != nil {
		return Message{}, err
	}

	cacheMessage(&resp, r.Cache)
	return resp, nil

}

func rateLimitDeleteMessage(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:message:delete:%d", channelID),
		Limit:  20,
		Window: 10 * time.Second,
	}
}

func (r *REST) DeleteMessage(ctx context.Context, channelID ID, msgID ID) error {
	uncacheMessage(channelID, msgID, r.Cache)

	return r.RequestNoContent(ctx, RESTRequest{
		Method:    "DELETE",
		Path:      fmt.Sprintf("/v1/channels/%d/messages/%d", channelID, msgID),
		RateLimit: rateLimitDeleteMessage(channelID),
	})
}

func rateLimitBulkDeleteMessages(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:message:bulk_delete:%d", channelID),
		Limit:  10,
		Window: 10 * time.Second,
	}
}

func (r *REST) BulkDeleteMessages(ctx context.Context, channelID ID, messageIDs []ID) error {
	uncacheMessages(channelID, messageIDs, r.Cache)

	var payload struct {
		MessageIDs []ID `json:"message_ids"`
	}
	payload.MessageIDs = messageIDs

	return r.RequestNoContent(ctx, RESTRequest{
		Method:    "POST",
		Path:      fmt.Sprintf("/v1/channels/%d/messages/bulk-delete", channelID),
		Payload:   payload,
		RateLimit: rateLimitBulkDeleteMessages(channelID),
	})
}

func rateLimitChannelTyping(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:typing:%d", channelID),
		Limit:  20,
		Window: 10 * time.Second,
	}
}

func (r *REST) StartTyping(ctx context.Context, channelID ID) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method:    "POST",
		Path:      fmt.Sprintf("/v1/channels/%d/typing", channelID),
		RateLimit: rateLimitChannelTyping(channelID),
	})

}
