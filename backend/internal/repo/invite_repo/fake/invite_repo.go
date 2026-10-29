// Package fake contains an in-memory invite repository for tests.
package fake

import (
	"context"
	"sync"
	"time"
	"zvrtsh/internal/model"
)

type InviteRepo struct {
	mu      sync.Mutex
	invites map[string]model.Invite // code -> invite
}

func New() *InviteRepo {
	return &InviteRepo{invites: make(map[string]model.Invite)}
}

func (i *InviteRepo) CreateInvite(_ context.Context, invite model.Invite) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	for code, old := range i.invites {
		if old.UserName == invite.UserName {
			delete(i.invites, code)
		}
	}
	i.invites[invite.Code] = invite
	return nil
}

func (i *InviteRepo) TakeInvite(_ context.Context, code string) (model.Invite, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	invite, ok := i.invites[code]
	if !ok {
		return model.Invite{}, model.ErrNoInviteCode
	}
	delete(i.invites, code)
	return invite, nil
}

func (i *InviteRepo) DeleteInvite(_ context.Context, userName string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	for code, invite := range i.invites {
		if invite.UserName == userName {
			delete(i.invites, code)
		}
	}
	return nil
}

func (i *InviteRepo) DeleteExpiredInvites(_ context.Context, now time.Time) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	for code, invite := range i.invites {
		if !now.Before(invite.ExpiresAt) {
			delete(i.invites, code)
		}
	}
	return nil
}

// Len returns the number of stored invites.
func (i *InviteRepo) Len() int {
	i.mu.Lock()
	defer i.mu.Unlock()
	return len(i.invites)
}
