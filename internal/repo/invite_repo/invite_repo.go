package inviterepo

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
)

type inviteRepoImpl struct {
	mu      *sync.Mutex
	invites map[string]uint64
}

func (i *inviteRepoImpl) CreateInviteCode(_ context.Context, inviteCode string, id uint64) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, ok := i.invites[inviteCode]; ok {
		return model.ErrCollision
	}

	i.invites[inviteCode] = id
	return nil
}

func (i *inviteRepoImpl) GetId(_ context.Context, inviteCode string) (uint64, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	id, ok := i.invites[inviteCode]
	if !ok {
		return 0, model.ErrNoInviteCode
	}
	return id, nil
}
