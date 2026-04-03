package flo

import (
	"context"
	"fmt"
	"time"
)

// SendMessageOpts specifies a message to send.
type SendMessageOpts struct {
	Content string
	Embeds  []EmbedOpts
}

// EmbedOpts specifies a rich embed when sending or editing a message.
type EmbedOpts struct {
	URL         string
	Title       string
	Color       ColorInt
	Timestamp   time.Time
	Description string
	Author      EmbedAuthorOpts
	Image       EmbedMediaOpts
	Thumbnail   EmbedMediaOpts
	Footer      EmbedFooterOpts
	Fields      []EmbedField
}

// EmbedAuthorOpts specifies an embed author when sending or editing a message.
type EmbedAuthorOpts struct {
	Name    string
	URL     string
	IconURL string
}

// EmbedAuthorOpts specifies embed media when sending or editing a message.
type EmbedMediaOpts struct {
	URL         string
	Description string
}

// EmbedAuthorOpts specifies an embed footer when sending or editing a message.
type EmbedFooterOpts struct {
	Text    string
	IconURL string
}

func (r *REST) SendMessage(ctx context.Context, channel ID, opts SendMessageOpts) (Message, error) {
	var resp rawMessage
	err := r.RequestJSON(ctx, RESTRequest{
		Method:  "POST",
		Path:    fmt.Sprintf("/v1/channels/%d/messages", channel),
		Bucket:  fmt.Sprintf("channel:message:create:%d", channel),
		Payload: opts.toRaw(),
	}, &resp)
	if err != nil {
		return Message{}, err
	}

	var msg Message
	msg.ID = resp.ID
	msg.update(r.Cache, &resp)

	return msg, nil
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
