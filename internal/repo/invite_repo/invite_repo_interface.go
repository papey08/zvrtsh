package inviterepo

import (
	"context"
	"sync"
)

type InviteRepo interface {
	CreateInviteCode(ctx context.Context, inviteCode string, id uint64) error
	GetId(ctx context.Context, inviteCode string) (uint64, error)
}

func New() InviteRepo {
	return &inviteRepoImpl{
		mu:      new(sync.Mutex),
		invites: make(map[string]uint64),
	}
}
