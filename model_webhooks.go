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

// Auth returns a [WebhookAuth] value which can be used to perform actions
func (w *Webhook) Auth() WebhookAuth {
	return WebhookAuth{w.ID, w.Token}
}

func (w *Webhook) Update(ctx context.Context, rest *REST, opts UpdateWebhookOpts) error {
	webhook, err := rest.UpdateWebhook(ctx, w.ID, opts)
	if err != nil {
		return err
	}

	*w = webhook
	return nil
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
