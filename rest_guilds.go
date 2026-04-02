package flo

import (
	"context"
	"fmt"
)

func (r *REST) GetGuild(ctx context.Context, id ID) (Guild, error) {
	var resp rawGuild
	err := r.RequestJSON(ctx, &resp, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d", id),
		Bucket: fmt.Sprintf("guild:read:%d", id),
	})
	if err != nil {
		return Guild{}, err
	}

	var guild Guild

	if r.Cache.Guilds != nil {
		hit := r.Cache.Guilds.Update(resp.ID, func(cached *Guild) {
			cached.update(resp)
			guild = *cached
		})
		if hit {
			return guild, nil
		}
	}

	guild.ID = id
	guild.update(resp)
	return guild, nil
}
