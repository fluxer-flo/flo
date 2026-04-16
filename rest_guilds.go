package flo

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"time"
)

func (r *REST) GetGuild(ctx context.Context, guildID ID) (Guild, error) {
	var result Guild
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d", guildID),
		Bucket: fmt.Sprintf("guild:read:%d", guildID),
	}, &result)
	if err != nil {
		return Guild{}, err
	}

	cacheGuild(&result, r.Cache)
	return result, nil
}

type CreateGuildChannelOpts struct {
	Type     ChannelType `json:"type"`
	Name     string      `json:"name,omitempty"`
	Topic    string      `json:"topic,omitempty"`
	URL      string      `json:"url,omitempty"`
	ParentID ID          `json:"parent_id,omitempty"`
	Bitrate  int         `json:"bitrate,omitzero"`
	NSFW     bool        `json:"nsfw,omitzero"`
}

func (r *REST) CreateGuildChannel(ctx context.Context, guildID ID, opts CreateGuildChannelOpts) (Channel, error) {
	var resp Channel
	err := r.RequestJSON(ctx, RESTRequest{
		Method:  "POST",
		Path:    fmt.Sprintf("/v1/guilds/%d/channels", guildID),
		Bucket:  fmt.Sprintf("guild:channel:create:%d", guildID),
		Payload: opts,
	}, &resp)
	if err != nil {
		return Channel{}, err
	}

	cacheChannel(&resp, r.Cache)
	return resp, nil
}

type CreateRoleOpts struct {
	Name  string   `json:"name"`
	Color ColorInt `json:"color"`
	Perms *Perms   `json:"permissions,omitempty"`
}

func (r *REST) CreateRole(ctx context.Context, guildID ID, opts CreateRoleOpts) (Role, error) {
	var resp Role
	err := r.RequestJSON(ctx, RESTRequest{
		Method:  "POST",
		Path:    fmt.Sprintf("/v1/guilds/%d/roles", guildID),
		Bucket:  fmt.Sprintf("guild:role:create:%d", guildID),
		Payload: opts,
	}, &resp)
	if err != nil {
		return Role{}, err
	}

	if r.Cache != nil {
		if guild, ok := r.Cache.Guilds.Get(guildID); ok {
			guild.Roles.Set(resp.ID, resp)
		}
	}

	return resp, nil
}

type GetMembersOpts struct {
	// Limit is specified as limit=... in the URL if not 0.
	Limit int
	// After is specified as after=... in the URL if not 0.
	After ID
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
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d/members", guildID),
		Query:  query.Encode(),
		Bucket: fmt.Sprintf("guild:members:%d", guildID),
	}, &resp)
	if err != nil {
		return nil, err
	}

	cacheMembers(guildID, resp, r.Cache)
	return resp, nil
}

func (r *REST) GetMember(ctx context.Context, guildID ID, userID ID) (Member, error) {
	var resp Member
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d/members/%d", guildID, userID),
		Bucket: fmt.Sprintf("guild:members:%d", guildID),
	}, &resp)
	if err != nil {
		return Member{}, err
	}

	cacheMember(guildID, &resp, r.Cache)
	return resp, nil
}

func (r *REST) GetCurrentMember(ctx context.Context, guildID ID) (Member, error) {
	var resp Member
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d/members/@me", guildID),
		Bucket: fmt.Sprintf("guild:members:%d", guildID),
	}, &resp)
	if err != nil {
		return Member{}, err
	}

	cacheMember(guildID, &resp, r.Cache)
	return resp, nil
}

type UpdateMemberOpts struct {
	AuditLogReason    string     `json:"-"`
	Nick              *string    `json:"nick,omitempty"`
	Roles             []ID       `json:"roles,omitzero"`
	Mute              *bool      `json:"mute,omitempty"`
	Deaf              *bool      `json:"deaf,omitempty"`
	CommDisabledUntil *time.Time `json:"communication_disabled_until,omitempty"`
	TimeoutReason     *string    `json:"timeout_reason,omitempty"`
	ChannelID         *ID        `json:"channel_id,omitempty"`
}

func (r *REST) UpdateMember(ctx context.Context, guildID ID, userID ID, opts UpdateMemberOpts) (Member, error) {
	var resp Member
	err := r.RequestJSON(ctx, RESTRequest{
		Method:         "PATCH",
		Path:           fmt.Sprintf("/v1/guilds/%d/members/%d", guildID, userID),
		Bucket:         fmt.Sprintf("guild:member:update:%d", guildID),
		Payload:        opts,
		AuditLogReason: opts.AuditLogReason,
	}, &resp)
	if err != nil {
		return Member{}, err
	}

	cacheMember(guildID, &resp, r.Cache)
	return resp, nil
}

type UpdateCurrentMemberOpts struct {
	AuditLogReason    string     `json:"-"`
	Nick              *string    `json:"nick,omitempty"`
	Avatar            *string    `json:"avatar,omitempty"`
	Banner            *string    `json:"banner,omitempty"`
	Bio               *string    `json:"bio,omitempty"`
	Pronouns          *string    `json:"pronouns,omitempty"`
	AccentColor       *ColorInt  `json:"accent_color,omitempty"`
	Mute              *bool      `json:"mute,omitempty"`
	Deaf              *bool      `json:"deaf,omitempty"`
	CommDisabledUntil *time.Time `json:"communication_disabled_until,omitempty"`
	TimeoutReason     *string    `json:"timeout_reason,omitempty"`
	ChannelID         *ID        `json:"channel_id,omitempty"`
}

func (r *REST) UpdateCurrentMember(ctx context.Context, guildID ID, opts UpdateCurrentMemberOpts) (Member, error) {
	var resp Member
	err := r.RequestJSON(ctx, RESTRequest{
		Method:         "PATCH",
		Path:           fmt.Sprintf("/v1/guilds/%d/members/@me", guildID),
		Bucket:         fmt.Sprintf("guild:member:update:%d", guildID),
		Payload:        opts,
		AuditLogReason: opts.AuditLogReason,
	}, &resp)
	if err != nil {
		return Member{}, err
	}

	cacheMember(guildID, &resp, r.Cache)
	return resp, nil
}

func (r *REST) AddMemberRole(ctx context.Context, guildID ID, userID ID, roleID ID) error {
	return r.AddMemberRoleWithReason(ctx, guildID, userID, roleID, "")
}

func (r *REST) AddMemberRoleWithReason(ctx context.Context, guildID ID, userID ID, roleID ID, reason string) error {
	err := r.RequestNoContent(ctx, RESTRequest{
		Method:         "PUT",
		Path:           fmt.Sprintf("/v1/guilds/%d/members/%d/roles/%d", guildID, userID, roleID),
		Bucket:         fmt.Sprintf("guild:member:role:add:%d", guildID),
		AuditLogReason: reason,
	})
	if err != nil {
		return err
	}

	if r.Cache != nil {
		if guild, ok := r.Cache.Guilds.Get(guildID); ok {
			if guild.Members != nil {
				guild.Members.Update(userID, func(member *Member) {
					oldRoles := member.Roles

					// NOTE: cloning just to be extra thread-safe (or thread cautious..?)
					member.Roles = make([]ID, 0, len(oldRoles)+1)
					member.Roles = append(member.Roles, oldRoles...)
					member.Roles = append(member.Roles, roleID)
				})
			}
		}
	}

	return nil
}

func (r *REST) RemoveMemberRole(ctx context.Context, guildID ID, userID ID, roleID ID) error {
	return r.RemoveMemberRoleWithReason(ctx, guildID, userID, roleID, "")
}

func (r *REST) RemoveMemberRoleWithReason(ctx context.Context, guildID ID, userID ID, roleID ID, reason string) error {
	err := r.RequestNoContent(ctx, RESTRequest{
		Method:         "DELETE",
		Path:           fmt.Sprintf("/v1/guilds/%d/members/%d/roles/%d", guildID, userID, roleID),
		Bucket:         fmt.Sprintf("guild:member:role:remove:%d", guildID),
		AuditLogReason: reason,
	})
	if err != nil {
		return err
	}

	if r.Cache != nil {
		if guild, ok := r.Cache.Guilds.Get(guildID); ok {
			if guild.Members != nil {
				guild.Members.Update(userID, func(member *Member) {
					oldRoles := member.Roles
					idx := slices.Index(oldRoles, roleID)
					if idx == -1 {
						return
					}

					// NOTE: cloning just to be extra thread-safe (or thread cautious..?)
					member.Roles = make([]ID, 0, len(oldRoles)-1)
					member.Roles = append(member.Roles, oldRoles[:idx]...)
					member.Roles = append(member.Roles, oldRoles[idx+1:]...)
				})
			}
		}
	}

	return nil
}

func (r *REST) RemoveMember(ctx context.Context, guildID ID, userID ID) error {
	return r.RemoveMemberWithReason(ctx, guildID, userID, "")
}

func (r *REST) RemoveMemberWithReason(ctx context.Context, guildID ID, userID ID, reason string) error {
	return r.RequestNoContent(ctx, RESTRequest{
		Method:         "DELETE",
		Path:           fmt.Sprintf("/v1/guilds/%d/members/%d", guildID, userID),
		Bucket:         fmt.Sprintf("guild:member:remove:%d", guildID),
		AuditLogReason: reason,
	})
}

func (r *REST) GetGuildBans(ctx context.Context, guildID ID) ([]GuildBan, error) {
	var resp []GuildBan
	err := r.RequestJSON(ctx, RESTRequest{
		Method: "GET",
		Path:   fmt.Sprintf("/v1/guilds/%d/bans", guildID),
		Bucket: fmt.Sprintf("guild:members:%d", guildID),
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
		Bucket:         fmt.Sprintf("guild:member:remove:%d", guildID),
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
		Bucket:         fmt.Sprintf("guild:member:remove:%d", guildID),
		AuditLogReason: reason,
	})
}
