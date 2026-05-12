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

// Path returns the path of the webhook URL with a leading slash.
// It can be appended to the base API URL to form the webhook URL.
func (a WebhookAuth) Path() string {
	return fmt.Sprintf("/webhooks/%d/%s", a.ID, url.PathEscape(a.Token))
}

func (a WebhookAuth) pathRedacted() string {
	return fmt.Sprintf("/webhooks/%d/token-redacted", a.ID)
}

// GetWebhook gets the webhook by the provided ID, so long as the authenticated user has [PermManageWebhooks].
func (r *REST) GetWebhook(ctx context.Context, webhookID ID) (Webhook, error) {
	var resp Webhook
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/webhooks/%d", webhookID),
		Bucket: fmt.Sprintf("webhook:read:%d", webhookID),
	}, &resp)
	if err != nil {
		return Webhook{}, err
	}

	return resp, nil

}

// GetWebhookWithAuth gets the webhook by the provided ID using its token.
// [Webhook.User] is not returned in the response.
func (r *REST) GetWebhookWithAuth(ctx context.Context, auth WebhookAuth) (Webhook, error) {
	var resp Webhook
	err := r.RequestJSON(ctx, RESTRequest{
		Method:       "GET",
		Path:         "/v1" + auth.Path(),
		RedactedPath: "/v1" + auth.pathRedacted(),
		Bucket:       fmt.Sprintf("webhook:read:%d", auth.ID),
	}, &resp)
	if err != nil {
		return Webhook{}, err
	}

	return resp, nil
}

// GetGuildWebhooks gets a list of guild webhooks, so long as the authenticated user has [PermManageWebhooks].
func (r *REST) GetGuildWebhooks(ctx context.Context, channelID ID) ([]Webhook, error) {
	var resp []Webhook
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d/webhooks", channelID),
		Bucket: fmt.Sprintf("webhook:list:%d", channelID),
	}, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// GetGuildWebhooks gets a list of channel webhooks, so long as the authenticated user has [PermManageWebhooks].
func (r *REST) GetChannelWebhooks(ctx context.Context, channelID ID) ([]Webhook, error) {
	var resp []Webhook
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/channels/%d/webhooks", channelID),
		Bucket: fmt.Sprintf("webhook:list:%d", channelID),
	}, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type CreateWebhookOpts struct {
	Name string `json:"name"`
	// Avatar is the webhook avatar in base64.
	Avatar string `json:"avatar,omitempty"`
}

func (r *REST) CreateWebhook(ctx context.Context, channelID ID, opts CreateWebhookOpts) (Webhook, error) {
	var resp Webhook
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "POST",
		Path:   fmt.Sprintf("/v1/channels/%d/webhooks", channelID),
		Bucket: fmt.Sprintf("webhook:create:%d", channelID),
	}, &resp)
	if err != nil {
		return Webhook{}, err
	}

	return resp, nil
}

type UpdateWebhookOpts struct {
	Name *string `json:"name,omitempty"`
	// Avatar is the webhook avatar in base64.
	Avatar *string `json:"avatar,omitempty"`
}

func (r *REST) UpdateWebhook(ctx context.Context, webhookID ID, opts UpdateWebhookOpts) (Webhook, error) {
	var resp Webhook
	err := r.RequestJSON(ctx, RESTRequest{
		Method:  "PATCH",
		Path:    fmt.Sprintf("/v1/webhooks/%d", webhookID),
		Bucket:  fmt.Sprintf("webhook:update:%d", webhookID),
		Payload: opts,
	}, &resp)
	if err != nil {
		return Webhook{}, err
	}

	return resp, nil
}

// UpdateWebhookWithAuth updates the properties of the specified webhook using its token.
// [Webhook.User] is not returned in the response.
func (r *REST) UpdateWebhookWithAuth(ctx context.Context, auth WebhookAuth, opts UpdateWebhookOpts) (Webhook, error) {
	var resp Webhook
	err := r.RequestJSON(ctx, RESTRequest{
		Method:       "PATCH",
		Path:         "/v1" + auth.Path(),
		RedactedPath: "/v1" + auth.pathRedacted(),
		Bucket:       fmt.Sprintf("webhook:update:%d", auth.ID),
		Payload:      opts,
	}, &resp)
	if err != nil {
		return Webhook{}, err
	}

	return resp, nil
}

func (r *REST) DeleteWebhook(ctx context.Context, webhookID ID) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method: "DELETE",
		Path:   fmt.Sprintf("/v1/webhooks/%d", webhookID),
		Bucket: fmt.Sprintf("webhook:delete:%d", webhookID),
	})
}

func (r *REST) DeleteWebhookWithAuth(ctx context.Context, auth WebhookAuth) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method:       "DELETE",
		Path:         "/v1" + auth.Path(),
		RedactedPath: "/v1" + auth.pathRedacted(),
		Bucket:       fmt.Sprintf("webhook:delete:%d", auth.ID),
	})
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

	return r.RequestNoContent(ctx, RESTRequest{
		Method:       "POST",
		Path:         "/v1" + auth.Path(),
		RedactedPath: "/v1" + auth.pathRedacted(),
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

	var resp Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method:       "POST",
		Path:         "/v1" + auth.Path(),
		RedactedPath: "/v1" + auth.pathRedacted(),
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
	Embeds          []EmbedOpts      `json:"embeds,omitzero"`
	AllowedMentions *AllowedMentions `json:"allowed_mentions,omitempty"`
	Flags           *MessageFlags    `json:"flags,omitempty"`
}

// EditWebhookMessage edits a message sent by a webhook using its token.
func (r *REST) EditWebhookMessage(ctx context.Context, auth WebhookAuth, messageID ID, opts EditWebhookMessageOpts) (Message, error) {
	if opts.AllowedMentions == nil {
		opts.AllowedMentions = r.DefaultAllowedMentions
	}

	var resp Message
	err := r.RequestJSON(ctx, RESTRequest{
		Method:       "PATCH",
		Path:         fmt.Sprintf("/v1%s/messages/%d", auth.Path(), messageID),
		RedactedPath: fmt.Sprintf("/v1%s/messages/%d", auth.pathRedacted(), messageID),
		Bucket:       fmt.Sprintf("webhook:message_edit:%d", auth.ID),
		Payload:      opts,
	}, &resp)
	if err != nil {
		return Message{}, err
	}

	cacheMessage(&resp, r.Cache)
	return resp, nil
}

// DeleteWebhookMessage deletes a message sent by a webhook using its token.
func (r *REST) DeleteWebhookMessage(ctx context.Context, auth WebhookAuth, messageID ID) error {
	const pathFmt = "/v1/webhooks/%d/%s/messages/%d"

	return r.RequestNoContent(ctx, RESTRequest{
		Method:       "DELETE",
		Path:         fmt.Sprintf("/v1%s/messages/%d", auth.Path(), messageID),
		RedactedPath: fmt.Sprintf("/v1%s/messages/%d", auth.pathRedacted(), messageID),
		Bucket:       fmt.Sprintf("webhook:message_delete:%d", auth.ID),
	})
}
