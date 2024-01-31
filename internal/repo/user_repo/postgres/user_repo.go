package postgres

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"zvrtsh/internal/model"
)

const (
	createUserQuery = `
		INSERT INTO "users" ("name", "encoded_password", "secret_code")
		VALUES ($1, $2, $3);`

	getUser = `
		SELECT * FROM "users"
		WHERE "name" = $1;`
)

type UserRepoImpl struct {
	*pgx.Conn
}

func (u *UserRepoImpl) CreateUser(ctx context.Context, name string, encodedPassword string, SecretCode string) (model.User, error) {
	_, err := u.Exec(ctx, createUserQuery,
		name,
		encodedPassword,
		SecretCode,
	)
	var pgxErr *pgconn.PgError
	if errors.As(err, &pgxErr) && pgxErr.Code == "23505" { // code of duplicate key error
		return model.User{}, model.ErrUserExists
	} else if err != nil {
		return model.User{}, model.ErrDatabase
	}
	return model.User{
		Name:            name,
		EncodedPassword: encodedPassword,
		SecretCode:      SecretCode,
	}, nil
}

func (u *UserRepoImpl) GetUser(ctx context.Context, name string) (model.User, error) {
	row := u.QueryRow(ctx, getUser, name)
	var user model.User
	if err := row.Scan(&user.Name, &user.EncodedPassword, &user.SecretCode); errors.Is(err, pgx.ErrNoRows) {
		return model.User{}, model.ErrNoUser
	} else if err != nil {
		return model.User{}, model.ErrDatabase
	} else {
		return user, nil
	}
}

func (u *UserRepoImpl) Close() {
	_ = u.Conn.Close(context.Background())
}
