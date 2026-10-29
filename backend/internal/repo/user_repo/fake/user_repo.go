// Package fake contains an in-memory user repository for tests.
package fake

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
)

type UserRepo struct {
	mu    sync.Mutex
	users map[string]model.User
}

func New() *UserRepo {
	return &UserRepo{users: make(map[string]model.User)}
}

func (u *UserRepo) CreateUser(_ context.Context, user model.User) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if _, ok := u.users[user.Name]; ok {
		return model.ErrUserExists
	}
	u.users[user.Name] = user
	return nil
}

func (u *UserRepo) GetUser(_ context.Context, name string) (model.User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	user, ok := u.users[name]
	if !ok {
		return model.User{}, model.ErrUserNotFound
	}
	return user, nil
}

func (u *UserRepo) GetUserByToken(_ context.Context, token string) (model.User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	for _, user := range u.users {
		if user.Token == token {
			return user, nil
		}
	}
	return model.User{}, model.ErrUserNotFound
}
