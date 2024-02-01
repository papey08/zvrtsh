// deprecated
package inmemory

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
)

type UserRepoImpl struct {
	Mu    *sync.Mutex
	Users map[string]*model.User
}

func (u *UserRepoImpl) CreateUser(_ context.Context, name string, encodedPassword string, SecretCode string) (model.User, error) {
	u.Mu.Lock()
	defer u.Mu.Unlock()

	if _, ok := u.Users[name]; ok {
		return model.User{}, model.ErrUserExists
	}

	newUser := model.User{
		Name:            name,
		EncodedPassword: encodedPassword,
		SecretCode:      SecretCode,
	}

	u.Users[name] = &newUser
	return newUser, nil
}

func (u *UserRepoImpl) GetUser(_ context.Context, name string) (model.User, error) {
	u.Mu.Lock()
	defer u.Mu.Unlock()

	user, ok := u.Users[name]
	if !ok {
		return model.User{}, model.ErrNoUser
	}

	return *user, nil
}

func (u *UserRepoImpl) Close() {}
