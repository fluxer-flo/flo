package flo

import (
	"context"
	"fmt"
)

func (r *REST) GetGuild(ctx context.Context, id ID) (Guild, error) {
	var result Guild
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d", id),
		Bucket: fmt.Sprintf("guild:read:%d", id),
	}, &result)
	if err != nil {
		return Guild{}, err
	}

	if r.Cache.Guilds != nil {
		r.Cache.Guilds.Update(result.ID, func(guild *Guild) {
			guild.updateREST(&result)
			result = *guild
		})
	}

	return result, nil
}
