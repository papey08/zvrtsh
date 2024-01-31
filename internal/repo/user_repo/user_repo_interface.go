package userrepo

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
	"zvrtsh/internal/repo/user_repo/inmemory"
)

type UserRepo interface {
	CreateUser(ctx context.Context, name string, encodedPassword string, SecretCode uint64) (model.User, error)
	GetUser(ctx context.Context, name string) (model.User, error)
}

func New() UserRepo {
	return &inmemory.UserRepoImpl{
		Mu:    new(sync.Mutex),
		Users: make(map[string]*model.User),
	}
}
