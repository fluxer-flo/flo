package flo

import (
	"context"
	"fmt"
	"time"
)

func rateLimitReadUser(userID ID) RateLimitConfig {
	return RateLimitConfig{
		Bucket: fmt.Sprintf("users:read:%d", userID),
		Limit:  100,
		Window: 10 * time.Second,
	}
}

func (r *REST) GetUser(ctx context.Context, userID ID) (User, error) {
	var result User
	err := r.RequestJSON(ctx, RESTRequest{
		Path:      fmt.Sprintf("/users/%d", userID),
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
