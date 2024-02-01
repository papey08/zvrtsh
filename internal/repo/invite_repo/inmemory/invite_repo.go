package inmemory

import (
	"context"
	"sync"
	"time"
	"zvrtsh/internal/model"
)

type InviteRepoImpl struct {
	Mu      *sync.Mutex
	Invites map[string]model.Invite
}

func (i *InviteRepoImpl) CreateInviteCode(_ context.Context, inviteCode string, _ string) error {
	i.Mu.Lock()
	defer i.Mu.Unlock()

	if _, ok := i.Invites[inviteCode]; ok {
		return model.ErrCollision
	}

	i.Invites[inviteCode] = model.Invite{
		Code:         inviteCode,
		CreationDate: time.Now().UTC(),
	}
	return nil
}

func (i *InviteRepoImpl) GetId(_ context.Context, inviteCode string) (string, error) {
	i.Mu.Lock()
	defer i.Mu.Unlock()

	id, ok := i.Invites[inviteCode]
	if !ok {
		return "", model.ErrNoInviteCode
	}
	return id.Code, nil
}

func (i *InviteRepoImpl) PurgeInviteCodes() {
	go func() {
		time.Sleep(time.Minute * 30)
		toRemove := make([]string, len(i.Invites))
		for k, v := range i.Invites {
			if v.CreationDate.Add(time.Hour).Before(time.Now().UTC()) {
				toRemove = append(toRemove, k)
			}
		}
		for _, k := range toRemove {
			delete(i.Invites, k)
		}
	}()
}
