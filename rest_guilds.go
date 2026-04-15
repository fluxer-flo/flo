package flo

import (
	"context"
	"fmt"
	"net/url"
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

type GetMembersOpts struct {
	// Limit is specified as limit=... in the URL if not 0.
	Limit int
	// After is specified as after=... in the URL if not 0.
	After ID
}

func rateLimitGuildMembers(guildID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("guild:members:%d", guildID),
		Limit:  40,
		Window: 10 * time.Second,
	}
}

func (r *REST) GetMembers(ctx context.Context, guildID ID, opts GetMembersOpts) ([]Member, error) {
	query := url.Values{}
	if opts.After != 0 {
		query.Set("after", fmt.Sprint(opts.After))
	}
	if opts.Limit != 0 {
		query.Set("limit", fmt.Sprint(opts.Limit))
	}

	var resp []Member
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "GET",
		Path:      fmt.Sprintf("/v1/guilds/%d/members", guildID),
		Query:     query.Encode(),
		RateLimit: rateLimitGuildMembers(guildID),
	}, &resp)
	if err != nil {
		return nil, err
	}

	cacheMembers(guildID, resp, r.Cache)
	return resp, nil
}

func rateLimitMemberRemove(guildID ID) RESTRateLimitConfig {
	return RESTRateLimitConfig{
		Bucket: fmt.Sprintf("guild:member:remove:%d", guildID),
		Limit:  20,
		Window: 10 * time.Second,
	}
}

func (r *REST) RemoveMember(ctx context.Context, guildID ID, userID ID) error {
	return r.RemoveMemberWithReason(ctx, guildID, userID, "")
}

func (r *REST) RemoveMemberWithReason(ctx context.Context, guildID ID, userID ID, reason string) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method:         "DELETE",
		Path:           fmt.Sprintf("/v1/guilds/%d/members/%d", guildID, userID),
		RateLimit:      rateLimitMemberRemove(guildID),
		AuditLogReason: reason,
	})
}

func (r *REST) GetGuildBans(ctx context.Context, guildID ID) ([]GuildBan, error) {
	var resp []GuildBan
	err := r.RequestJSON(ctx, RESTRequest{
		Method:    "GET",
		Path:      fmt.Sprintf("/v1/guilds/%d/bans", guildID),
		RateLimit: rateLimitGuildMembers(guildID),
	}, &resp)
	if err != nil {
		return nil, err
	}

	if r.Cache != nil {
		for _, ban := range resp {
			r.Cache.Users.Set(ban.User.ID, ban.User)
		}
	}

	return resp, nil
}

type CreateGuildBanOpts struct {
	// DeleteMessageDays is the days worth of messages to delete (0-7).
	DeleteMessageDays uint
	// Reason specifies the reason which appears in the ban list.
	Reason string
	// AuditLogReason specifies an audit-log specific reason which does not persist.
	AuditLogReason string
	// BanDuration specifies how long to ban the user for if not 0.
	// It is converted to seconds so anything more precise will be lost.
	BanDuration time.Duration
}

func (r *REST) CreateGuildBan(ctx context.Context, guildID ID, userID ID, opts CreateGuildBanOpts) error {
	var payload struct {
		DeleteMessageDays  uint   `json:"delete_message_days,omitempty"`
		Reason             string `json:"reason,omitempty"`
		BanDurationSeconds int64  `json:"ban_duration_seconds,omitempty"`
	}
	payload.DeleteMessageDays = opts.DeleteMessageDays
	payload.Reason = opts.Reason
	payload.BanDurationSeconds = int64(opts.BanDuration / time.Second)

	return r.RequestNoContent(ctx, RESTRequest{
		Method:         "PUT",
		Path:           fmt.Sprintf("/v1/guilds/%d/bans/%d", guildID, userID),
		RateLimit:      rateLimitMemberRemove(guildID),
		Payload:        payload,
		AuditLogReason: opts.AuditLogReason,
	})
}

func (r *REST) RemoveGuildBan(ctx context.Context, guildID ID, userID ID) error {
	return r.RemoveGuildBanWithReason(ctx, guildID, userID, "")
}

func (r *REST) RemoveGuildBanWithReason(ctx context.Context, guildID ID, userID ID, reason string) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method:         "DELETE",
		Path:           fmt.Sprintf("/v1/guilds/%d/bans/%d", guildID, userID),
		RateLimit:      rateLimitMemberRemove(guildID),
		AuditLogReason: reason,
	})
}
