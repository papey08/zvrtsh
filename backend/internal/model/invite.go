package model

import "time"

type Invite struct {
	Code         string
	UserName     string
	StepsAmount  int // of the session started by the invite
	CreationDate time.Time
	ExpiresAt    time.Time
}
