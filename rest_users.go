package flo

import (
	"context"
	"fmt"
	"time"
)

func rateLimitReadUser(userID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("users:read:%d", userID),
		Limit:  100,
		Window: 10 * time.Second,
	}
}

func (r *REST) GetUser(ctx context.Context, userID ID) (User, error) {
	var result User
	err := r.RequestJSON(ctx, RESTRequest{
		Path:      fmt.Sprintf("/v1/users/%d", userID),
		RateLimit: rateLimitReadUser(userID),
	}, &result)
	if err != nil {
		return User{}, nil
	}

	if r.Cache != nil {
		r.Cache.Users.Set(result.ID, result)
	}

	return result, nil
}

var rateLimitGetUserSettings = RESTRateLimitConfig{
	Bucket: "users:settings:get",
	Limit:  40,
	Window: 10 * time.Second,
}

func (r *REST) GetCurrentUser(ctx context.Context) (UserPrivate, error) {
	var result UserPrivate
	err := r.RequestJSON(ctx, RESTRequest{
		Path:      "/v1/users/@me",
		RateLimit: rateLimitGetUserSettings,
	}, &result)
	if err != nil {
		return UserPrivate{}, nil
	}

	cacheCurrentUser(&result, r.Cache)
	return result, nil
}
