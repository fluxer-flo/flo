package flo

import (
	"context"
	"fmt"
	"time"
)

// SendMessageOpts specifies a message to send.
type SendMessageOpts struct {
	Content string      `json:"content,omitempty"`
	Embeds  []EmbedOpts `json:"embeds,omitempty"`
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
	URL     string `json:"url"`
	IconURL string  `json:"icon_url"`
}

// EmbedAuthorOpts specifies embed media when sending or editing a message.
type EmbedMediaOpts struct {
	URL         string `json:"url"`
	Description string `json:"description"`
}

// EmbedAuthorOpts specifies an embed footer when sending or editing a message.
type EmbedFooterOpts struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url"`
}

func (r *REST) SendMessage(ctx context.Context, channel ID, opts SendMessageOpts) (Message, error) {
	var resp Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method:  "POST",
		Path:    fmt.Sprintf("/v1/channels/%d/messages", channel),
		Bucket:  fmt.Sprintf("channel:message:create:%d", channel),
		Payload: opts,
	}, &resp)
	if err != nil {
		return Message{}, err
	}

	return resp, nil
}

func (r *REST) SendMessageContent(ctx context.Context, channel ID, content string) (Message, error) {
	return r.SendMessage(ctx, channel, SendMessageOpts{
		Content: content,
	})
}

func (r *REST) DeleteMessage(ctx context.Context, channel ID, message ID) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method: "DELETE",
		Path:   fmt.Sprintf("/v1/channels/%d/messages/%d", channel, message),
		Bucket: fmt.Sprintf("channel:message:delete:%d", channel),
	})
}
