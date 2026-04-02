package flo

import (
	"context"
	"fmt"
)

type SendMessageOpts struct {
	Content string
}

func (r *REST) SendMessage(ctx context.Context, channel ID, msg SendMessageOpts) (Message, error) {
	type rawSendMsg struct {
		Content string `json:"content"`
	}
	payload := rawSendMsg{
		Content: msg.Content,
	}

	_, err := r.Request(ctx, RESTRequest{
		Method:  "POST",
		Path:    fmt.Sprintf("/v1/channels/%d/messages", channel),
		Bucket:  fmt.Sprintf("channel:message:create::%d", channel),
		Payload: payload,
	})
	if err != nil {
		return Message{}, err
	}

	return Message{}, nil
}

func (r *REST) SendMessageContent(ctx context.Context, channel ID, msg string) (Message, error) {
	return r.SendMessage(ctx, channel, SendMessageOpts{
		Content: msg,
	})
}

func (r *REST) DeleteMessage(ctx context.Context, channel ID, message ID) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Path:   fmt.Sprintf("/v1/channels/%d/messages/%d", channel, message),
		Bucket: fmt.Sprintf("channel:message:delete:%d", channel),
	})
}
