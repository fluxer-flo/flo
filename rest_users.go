package flo

import (
	"context"
	"fmt"
)

func (r *REST) GetUser(ctx context.Context, userID ID) (User, error) {
	var result User
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/users/%d", userID),
		Bucket: fmt.Sprintf("users:read:%d", userID),
	}, &result)
	if err != nil {
		return User{}, nil
	}

	if r.Cache != nil {
		r.Cache.Users.Set(result.ID, result)
	}

	return result, nil
}

func (r *REST) GetCurrentUser(ctx context.Context) (UserPrivate, error) {
	var result UserPrivate
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   "/v1/users/@me",
		Bucket: "users:settings:get",
	}, &result)
	if err != nil {
		return UserPrivate{}, nil
	}

	cacheCurrentUser(&result, r.Cache)
	return result, nil
}

func (r *REST) CreateDMChannel(ctx context.Context, userID ID) (Channel, error) {
	var raw struct {
		RecipientID ID `json:"recipient_id"`
	}
	raw.RecipientID = userID

	var resp Channel
	err := r.RequestJSON(ctx, RESTRequest{
		Method:  "POST",
		Path:    "/v1/users/@me/channels",
		Payload: raw,
		Bucket:  "user:channels",
	}, &resp)
	if err != nil {
		return Channel{}, err
	}

	cacheChannel(&resp, r.Cache)
	return resp, nil

}

func (r *REST) LeaveGuild(ctx context.Context, guildID ID) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method: "DELETE",
		Path:   fmt.Sprintf("/v1/users/@me/guilds/%d", guildID),
		Bucket: fmt.Sprintf("guilds:leave:%d", guildID),
	})
}
