// deprecated
package inmemory

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
)

type InviteRepoImpl struct {
	Mu      *sync.Mutex
	Invites map[string]string
}

func (i *InviteRepoImpl) CreateInviteCode(_ context.Context, inviteCode string, id string) error {
	i.Mu.Lock()
	defer i.Mu.Unlock()

	if _, ok := i.Invites[inviteCode]; ok {
		return model.ErrCollision
	}

	i.Invites[inviteCode] = id
	return nil
}

func (i *InviteRepoImpl) GetId(_ context.Context, inviteCode string) (string, error) {
	i.Mu.Lock()
	defer i.Mu.Unlock()

	id, ok := i.Invites[inviteCode]
	if !ok {
		return "", model.ErrNoInviteCode
	}
	return id, nil
}
