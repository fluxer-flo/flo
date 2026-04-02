package flo

import "time"

type User struct {
	ID            ID
	Username      string
	Discriminator string
	GlobalName    *string
	Avatar        *string
	AvatarColor   *ColorInt
	Bot           bool
	System        bool
	Flags         UserFlags
}

func (u *User) CreatedAt() time.Time {
	return u.ID.CreatedAt()
}

func (u *User) Tag() string {
	return u.Username + "#" + u.Discriminator
}

func (u *User) DisplayName() string {
	if u.GlobalName != nil {
		return *u.GlobalName
	} else {
		return u.Username
	}
}

func (u *User) IsDeleted() bool {
	// this appears to be a reliable indicator of deleted user:
	// https://fluxer.app/channels/1427764813854588940/1483532018185537313/1489339598513306876
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

type UserNofifSettings uint

const (
	UserNotifSettingsAllMessages  UserNofifSettings = 0
	UserNotifSettingsOnlyMentions UserNofifSettings = 1
	UserNotifSettingsNoMessages   UserNofifSettings = 2
	UserNotifSettingsInherit      UserNofifSettings = 3
)
