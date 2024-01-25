package userrepo

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
)

type UserRepo interface {
	CreateUser(ctx context.Context, name string, encodedPassword string, SecretCode uint64) (model.User, error)
	GetUser(ctx context.Context, name string) (model.User, error)
}

func New() UserRepo {
	return &userRepoImpl{
		mu:    new(sync.Mutex),
		users: make(map[string]*model.User),
	}
}
