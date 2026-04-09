package flo

import (
	"context"
	"fmt"
	"time"
)

// CreateMessageOpts specifies a message to send.
type CreateMessageOpts struct {
	Content         string           `json:"content,omitempty"`
	Embeds          []EmbedOpts      `json:"embeds,omitempty"`
	AllowedMentions *AllowedMentions `json:"allowed_mentions,omitempty"`
}

// EmbedOpts specifies a rich embed when sending or editing a message.
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

// EmbedAuthorOpts specifies an embed author when sending or editing a message.
type EmbedAuthorOpts struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

// EmbedAuthorOpts specifies embed media when sending or editing a message.
type EmbedMediaOpts struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// EmbedAuthorOpts specifies an embed footer when sending or editing a message.
type EmbedFooterOpts struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

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

	var resp Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "POST",
		Path:      fmt.Sprintf("/v1/channels/%d/messages", channelID),
		RateLimit: rateLimitCreateMessage(channelID),
		Payload:   opts,
	}, &resp)
	if err != nil {
		return Message{}, err
	}

	if r.Cache != nil {
		resp.updateCache(r.Cache)
	}

	return resp, nil
}

func rateLimitDeleteMessage(channelID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("channel:message:delete:%d", channelID),
		Limit:  20,
		Window: 10 * time.Second,
	}
}

func (r *REST) DeleteMessage(ctx context.Context, channelID ID, messageID ID) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method:    "DELETE",
		Path:      fmt.Sprintf("/v1/channels/%d/messages/%d", channelID, messageID),
		RateLimit: rateLimitDeleteMessage(channelID),
	})
}
