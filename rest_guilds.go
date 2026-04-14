package flo

import (
	"context"
	"fmt"
	"time"
)

func rateLimitReadGuild(guildID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("guild:read:%d", guildID),
		Limit:  100,
		Window: 10 * time.Second,
	}
}

func (r *REST) GetGuild(ctx context.Context, guildID ID) (Guild, error) {
	var result Guild
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "GET",
		Path:      fmt.Sprintf("/v1/guilds/%d", guildID),
		RateLimit: rateLimitReadGuild(guildID),
	}, &result)
	if err != nil {
		return Guild{}, err
	}

	cacheGuild(&result, r.Cache)
	return result, nil
}
