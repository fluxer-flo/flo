package flo

import "time"

//go:generate stringer -type=UserNotifSettings -output=model_user_string.go

type User struct {
	ID            ID        `json:"id"`
	Username      string    `json:"username"`
	Discriminator string    `json:"discriminator"`
	GlobalName    *string   `json:"global_name"`
	Avatar        *string   `json:"avatar"`
	AvatarColor   *ColorInt `json:"avatar_color"`
	Bot           bool      `json:"bot"`
	System        bool      `json:"system"`
	Flags         UserFlags `json:"flags"`
}

func (u *User) CreatedAt() time.Time {
	return u.ID.CreatedAt()
}

// Tag returns a string of Username#Discriminator.
func (u *User) Tag() string {
	return u.Username + "#" + u.Discriminator
}

// DisplayName returns the rendered name in chat outside of any guilds.
func (u *User) DisplayName() string {
	if u.GlobalName != nil {
		return *u.GlobalName
	} else {
		return u.Username
	}
}

// IsDeleted returns true if the user is a placeholder for a deleted user.
// This is currently indicated by a tag of DeletedUser#0000.
func (u *User) IsDeleted() bool {
	// this appears to be a reliable indicator of deleted user:
	// https://web.fluxer.app/channels/1427764813854588940/1483532018185537313/1489339598513306876
	return u.Username == "DeletedUser" && u.Discriminator == "0000"
}

type UserFlags uint

const (
	UserFlagStaff                     UserFlags = 1 << 0
	UserFlagCTPMember                 UserFlags = 1 << 1
	UserFlagPartner                   UserFlags = 1 << 2
	UserFlagBugHunter                 UserFlags = 1 << 3
	UserFlagFriendlyBot               UserFlags = 1 << 4
	UserFlagFriendlyBotManualApproval UserFlags = 1 << 5
)

type UserNotifSettings uint

const (
	UserNotifSettingsAllMessages  UserNotifSettings = 0
	UserNotifSettingsOnlyMentions UserNotifSettings = 1
	UserNotifSettingsNoMessages   UserNotifSettings = 2
	UserNotifSettingsInherit      UserNotifSettings = 3
)
