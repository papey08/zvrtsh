package inmemory

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
)

type InviteRepoImpl struct {
	Mu      *sync.Mutex
	Invites map[string]uint64
}

func (i *InviteRepoImpl) CreateInviteCode(_ context.Context, inviteCode string, id uint64) error {
	i.Mu.Lock()
	defer i.Mu.Unlock()

	if _, ok := i.Invites[inviteCode]; ok {
		return model.ErrCollision
	}

	i.Invites[inviteCode] = id
	return nil
}

func (i *InviteRepoImpl) GetId(_ context.Context, inviteCode string) (uint64, error) {
	i.Mu.Lock()
	defer i.Mu.Unlock()

	id, ok := i.Invites[inviteCode]
	if !ok {
		return 0, model.ErrNoInviteCode
	}
	return id, nil
}
