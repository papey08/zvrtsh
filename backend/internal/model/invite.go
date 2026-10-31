package model

import "time"

// Invite is the code by which players join a session in the lobby.
type Invite struct {
	Code      string
	ExpiresAt time.Time
}
