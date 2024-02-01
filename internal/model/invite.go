package model

import "time"

type Invite struct {
	Code         string
	CreationDate time.Time
}
