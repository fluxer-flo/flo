package flo

import (
	"context"
	"fmt"
	"net/url"
)

type WebhookAuth struct {
	ID    ID
	Token string
}

type ExecWebhookOpts struct {
	Content          string                 `json:"content,omitempty"`
	Embeds           []EmbedOpts            `json:"embeds,omitempty"`
	Attachments      []CreateAttachmentOpts `json:"attachments,omitempty"`
	MessageReference MessageReferenceOpts   `json:"message_reference,omitzero"`
	AllowedMentions  *AllowedMentions       `json:"allowed_mentions,omitempty"`
	Flags            MessageFlags           `json:"flags,omitzero"`
	Nonce            string                 `json:"nonce,omitempty"`
	StickerIDs       []ID                   `json:"sticker_ids,omitempty"`
	TTS              bool                   `json:"tts,omitzero"`
	Username         string                 `json:"username,omitempty"`
	AvatarURL        string                 `json:"avatar_url,omitempty"`
}

// ExecWebhook sends a message through a webhook without waiting.
// If you want a message repsonse use [ExecWebhookWait]
func (r *REST) ExecWebhook(ctx context.Context, auth WebhookAuth, opts ExecWebhookOpts) error {
	if opts.AllowedMentions == nil {
		opts.AllowedMentions = r.DefaultAllowedMentions
	}

	const pathFmt = "/v1/webhooks/%d/%s"

	return r.RequestNoContent(ctx, RESTRequest{
		Method: "POST",
		// NOTE: path escaping token to prevent abuse of .. (just in case)
		Path:         fmt.Sprintf(pathFmt, auth.ID, url.PathEscape(auth.Token)),
		RedactedPath: fmt.Sprintf(pathFmt, auth.ID, redact(auth.Token)),
		Bucket:       fmt.Sprintf("webhook:execute:%d", auth.ID),
		Payload:      opts,
		Form:         createAttachmentOptsToForm(opts.Attachments),
	})
}

// ExecWebhookWait sends a message through a webhook and returns the sent message.
func (r *REST) ExecWebhookWait(ctx context.Context, auth WebhookAuth, opts ExecWebhookOpts) (Message, error) {
	if opts.AllowedMentions == nil {
		opts.AllowedMentions = r.DefaultAllowedMentions
	}

	const pathFmt = "/v1/webhooks/%d/%s"

	var resp Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "POST",
		// NOTE: path escaping token to prevent abuse of .. (just in case)
		Path:         fmt.Sprintf(pathFmt, auth.ID, url.PathEscape(auth.Token)),
		RedactedPath: fmt.Sprintf(pathFmt, auth.ID, redact(auth.Token)),
		Query:        "wait=true",
		Bucket:       fmt.Sprintf("webhook:execute:%d", auth.ID),
		Payload:      opts,
		Form:         createAttachmentOptsToForm(opts.Attachments),
	}, &resp)
	if err != nil {
		return Message{}, err
	}

	cacheMessage(&resp, r.Cache)
	return resp, nil
}

// EditWebhookOpts specifies webhook message fields to edit.
// A field being left as nil indicates to keep it the same.
type EditWebhookMessageOpts struct {
	Content         *string          `json:"content,omitempty"`
	Embeds          []Embed          `json:"embeds,omitzero"`
	AllowedMentions *AllowedMentions `json:"allowed_mentions,omitempty"`
	Flags           *MessageFlags    `json:"flags,omitempty"`
}

// EditWebhookMessage edits a message sent by a webhook using its token.
func (r *REST) EditWebhookMessage(ctx context.Context, auth WebhookAuth, messageID ID, opts EditWebhookMessageOpts) (Message, error) {
	if opts.AllowedMentions == nil {
		opts.AllowedMentions = r.DefaultAllowedMentions
	}

	const pathFmt = "/v1/webhooks/%d/%s/messages/%d"

	var resp Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "PATCH",
		// NOTE: path escaping token to prevent abuse of .. (just in case)
		Path:         fmt.Sprintf(pathFmt, auth.ID, url.PathEscape(auth.Token), messageID),
		RedactedPath: fmt.Sprintf(pathFmt, auth.ID, redact(auth.Token), messageID),
		Bucket:       fmt.Sprintf("webhook:message_edit:%d", auth.ID),
		Payload:      opts,
	}, &resp)
	if err != nil {
		return Message{}, err
	}

	cacheMessage(&resp, r.Cache)
	return resp, nil
}

// EditWebhookMessage deletes a message sent by a webhook using its token.
func (r *REST) DeleteWebhookMessage(ctx context.Context, auth WebhookAuth, messageID ID) error {
	const pathFmt = "/v1/webhooks/%d/%s/messages/%d"

	return r.RequestNoContent(ctx, RESTRequest{
		Method: "DELETE",
		// NOTE: path escaping token to prevent abuse of .. (just in case)
		Path:         fmt.Sprintf(pathFmt, auth.ID, url.PathEscape(auth.Token), messageID),
		RedactedPath: fmt.Sprintf(pathFmt, auth.ID, redact(auth.Token), messageID),
		Bucket:       fmt.Sprintf("webhook:message_delete:%d", auth.ID),
	})

}
