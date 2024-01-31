package inviterepo

import (
	"context"
	"sync"

	"zvrtsh/internal/repo/invite_repo/inmemory"
)

type InviteRepo interface {
	CreateInviteCode(ctx context.Context, inviteCode string, id string) error
	GetId(ctx context.Context, inviteCode string) (string, error)
}

func New() InviteRepo {
	return &inmemory.InviteRepoImpl{
		Mu:      new(sync.Mutex),
		Invites: make(map[string]string),
	}
}
