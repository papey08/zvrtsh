package userrepo

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
)

type userRepoImpl struct {
	mu    *sync.Mutex
	users map[string]*model.User
}

func (u *userRepoImpl) CreateUser(_ context.Context, name string, encodedPassword string, SecretCode uint64) (model.User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if _, ok := u.users[name]; ok {
		return model.User{}, model.ErrUserExists
	}

	newUser := model.User{
		Name:            name,
		EncodedPassword: encodedPassword,
		SecretCode:      SecretCode,
	}

	u.users[name] = &newUser
	return newUser, nil
}

func (u *userRepoImpl) GetUser(ctx context.Context, name string) (model.User, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	user, ok := u.users[name]
	if !ok {
		return model.User{}, model.ErrNoUser
	}

	return *user, nil
}
