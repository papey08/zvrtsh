package model

import "time"

type Invite struct {
	InviteCode     string
	UserSecretCode string
	CreationDate   time.Time
}
