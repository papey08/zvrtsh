package userrepo

import (
	"context"
	"github.com/jackc/pgx/v5"
	"log"
	"time"
	"zvrtsh/internal/model"
	"zvrtsh/internal/repo/user_repo/postgres"
)

type UserRepo interface {
	CreateUser(ctx context.Context, name string, encodedPassword string, SecretCode string) (model.User, error)
	GetUser(ctx context.Context, name string) (model.User, error)
	Close()
}

func New(ctx context.Context, addr string) (UserRepo, error) {
	for i := 0; i < 30; i++ {
		conn, err := pgx.Connect(ctx, addr)
		if err != nil { // database haven't initialized in docker container yet
			log.Printf("taskRepo connection error: %s\n", err.Error())
			time.Sleep(time.Second)
		} else { // database already initialized
			return &postgres.UserRepoImpl{
				Conn: conn,
			}, nil
		}
	}
	return nil, model.ErrDatabase
}
