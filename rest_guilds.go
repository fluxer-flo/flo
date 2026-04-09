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
	result := newGuildForCache(guildID, r.Cache)
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "GET",
		Path:      fmt.Sprintf("/v1/guilds/%d", guildID),
		RateLimit: rateLimitReadGuild(guildID),
	}, &result)
	if err != nil {
		return Guild{}, err
	}

	if r.Cache != nil {
		hit := r.Cache.Guilds.Update(result.ID, func(guild *Guild) {
			guild.updateProperties(&result)
			result = *guild
		})
		if hit {
			return result, nil
		}
	}

	return result, nil
}
