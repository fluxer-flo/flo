package flo

import (
	"context"
	"fmt"
)

func (r *REST) GetUser(ctx context.Context, id ID) (User, error) {
	var result User
	err := r.RequestJSON(ctx, RESTRequest{
		Path:   fmt.Sprintf("/users/%d", id),
		Bucket: fmt.Sprintf("users:read:%d", id),
	}, &result)
	if err != nil {
		return User{}, nil
	}

	if r.Cache != nil {
		r.Cache.Users.Set(result.ID, result)
	}

	return result, nil
}
