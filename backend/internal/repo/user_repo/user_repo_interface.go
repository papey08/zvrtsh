package userrepo

import (
	"context"
	"zvrtsh/internal/model"
	"zvrtsh/internal/repo/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepo interface {
	CreateUser(ctx context.Context, user model.User) error
	GetUser(ctx context.Context, name string) (model.User, error)
	GetUserByToken(ctx context.Context, token string) (model.User, error)
}

func New(pool *pgxpool.Pool) UserRepo {
	return postgres.NewUserRepo(pool)
}
