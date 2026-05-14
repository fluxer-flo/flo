package flo

import (
	"context"
	"fmt"
	"net/url"
)

func (r *REST) GetInvite(ctx context.Context, code string) (Invite, error) {
	var resp Invite
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/invites/%s", url.PathEscape(code)),
		Bucket: fmt.Sprintf("invite:read:%s", code),
	}, &resp)
	if err != nil {
		return Invite{}, err
	}

	return resp, nil
}

func (r *REST) GetChannelInvites(ctx context.Context, guildID ID) ([]Invite, error) {
	var resp []Invite
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/channels/%d/invites", guildID),
		Bucket: fmt.Sprintf("invite:list:%d", guildID),
	}, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (r *REST) GetGuildInvites(ctx context.Context, guildID ID) ([]GuildInvite, error) {
	var resp []GuildInvite
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d/invites", guildID),
		Bucket: fmt.Sprintf("invite:list:%d", guildID),
	}, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type CreateChannelInviteOpts struct {
	// MaxUses is the maximum number of times the invite can be used (0 = unlimited).
	MaxUses    int `json:"max_uses"`
	// MaxAgeSecs is the duration in seconds that the invite will last for.
	MaxAgeSecs int `json:"max_age"`
	// Unique specifies whether to create a new invite or reuse an existing one.
	Unique bool `json:"unique"`
	// TempAccess specifies if the membership should last until the user goes offline.
	TempAccess bool `json:"temporary"`
}

func (r *REST) CreateChannelInvite(ctx context.Context, channelID ID, opts CreateChannelInviteOpts) (Invite, error) {
	var resp Invite
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "POST",
		Path:   fmt.Sprintf("/v1/channels/%d/invite", channelID),
		Bucket: fmt.Sprintf("invite:create:%d", channelID),
		Payload: opts,
	}, &resp)
	if err != nil {
		return Invite{}, err
	}

	return resp, nil
}

func (r *REST) DeleteInvite(ctx context.Context, code string) error {
	return r.DeleteInviteWithReason(ctx, code, "")
}

func (r *REST) DeleteInviteWithReason(ctx context.Context, code string, reason string) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method:         "DELETE",
		Path:           fmt.Sprintf("/v1/invites/%s", url.PathEscape(code)),
		Bucket:         fmt.Sprintf("invite:delete:%s", code),
		AuditLogReason: reason,
	})
}
