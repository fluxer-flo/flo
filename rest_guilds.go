package flo

import (
	"context"
	"fmt"
)

func (r *REST) GetGuild(ctx context.Context, id ID) (Guild, error) {
	var resp rawGuild
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d", id),
		Bucket: fmt.Sprintf("guild:read:%d", id),
	}, &resp)
	if err != nil {
		return Guild{}, err
	}

	return cacheGuild(r.Cache, &resp), nil
}
