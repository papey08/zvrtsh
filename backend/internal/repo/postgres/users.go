package postgres

import (
	"context"
	"errors"
	"zvrtsh/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	createUserQuery = `
		INSERT INTO "users" ("name", "encoded_password", "token")
		VALUES ($1, $2, $3);`

	getUserQuery = `
		SELECT "name", "encoded_password", "token" FROM "users"
		WHERE "name" = $1;`

	getUserByTokenQuery = `
		SELECT "name", "encoded_password", "token" FROM "users"
		WHERE "token" = $1;`
)

type UserRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (u *UserRepo) CreateUser(ctx context.Context, user model.User) error {
	_, err := u.pool.Exec(ctx, createUserQuery, user.Name, user.EncodedPassword, user.Token)

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
		return model.ErrUserExists
	} else if err != nil {
		return dbError(err)
	}
	return nil
}

func (u *UserRepo) GetUser(ctx context.Context, name string) (model.User, error) {
	return u.getUser(ctx, getUserQuery, name)
}

func (u *UserRepo) GetUserByToken(ctx context.Context, token string) (model.User, error) {
	return u.getUser(ctx, getUserByTokenQuery, token)
}

func (u *UserRepo) getUser(ctx context.Context, query string, arg string) (model.User, error) {
	var user model.User
	err := u.pool.QueryRow(ctx, query, arg).Scan(&user.Name, &user.EncodedPassword, &user.Token)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, model.ErrUserNotFound
	} else if err != nil {
		return model.User{}, dbError(err)
	}
	return user, nil
}
