package flo

import "context"

type Webhook struct {
	ID        ID      `json:"id"`
	GuildID   ID      `json:"guild_id"`
	ChannelID ID      `json:"channel_id"`
	Name      string  `json:"name"`
	Token     string  `json:"token"`
	User      *User   `json:"user"`
	Avatar    *string `json:"avatar"`
}

// Auth returns a [WebhookAuth] value which can be used to perform actions through the webhook.
func (w *Webhook) Auth() WebhookAuth {
	return WebhookAuth{w.ID, w.Token}
}

// Path returns the path of the webhook URL with a leading slash.
// It can be appended to the base API URL to form the webhook URL.
func (w *Webhook) Path() string {
	return w.Auth().Path()
}

func (w *Webhook) Update(ctx context.Context, rest *REST, opts UpdateWebhookOpts) error {
	webhook, err := rest.UpdateWebhookWithAuth(ctx, w.Auth(), opts)
	if err != nil {
		return err
	}

	oldUser := w.User
	*w = webhook
	w.User = oldUser
	return nil
}

func (w *Webhook) Delete(ctx context.Context, rest *REST) error {
	return rest.DeleteWebhookWithAuth(ctx, w.Auth())
}

func (w *Webhook) Exec(ctx context.Context, rest *REST, opts ExecWebhookOpts) error {
	return rest.ExecWebhook(ctx, w.Auth(), opts)
}

func (w *Webhook) ExecWait(ctx context.Context, rest *REST, opts ExecWebhookOpts) (Message, error) {
	return rest.ExecWebhookWait(ctx, w.Auth(), opts)
}

func (w *Webhook) EditMessage(ctx context.Context, rest *REST, messageID ID, opts EditWebhookMessageOpts) (Message, error) {
	return rest.EditWebhookMessage(ctx, w.Auth(), messageID, opts)
}

func (w *Webhook) DeleteMessage(ctx context.Context, rest *REST, messageID ID) error {
	return rest.DeleteWebhookMessage(ctx, w.Auth(), messageID)
}
