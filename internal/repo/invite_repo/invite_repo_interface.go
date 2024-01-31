package inviterepo

import (
	"context"
	"sync"

	"zvrtsh/internal/repo/invite_repo/inmemory"
)

type InviteRepo interface {
	CreateInviteCode(ctx context.Context, inviteCode string, id uint64) error
	GetId(ctx context.Context, inviteCode string) (uint64, error)
}

func New() InviteRepo {
	return &inmemory.InviteRepoImpl{
		Mu:      new(sync.Mutex),
		Invites: make(map[string]uint64),
	}
}
