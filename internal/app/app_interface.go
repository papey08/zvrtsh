package app

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
	inviterepo "zvrtsh/internal/repo/invite_repo"
	userrepo "zvrtsh/internal/repo/user_repo"
)

type App interface {
	sessionApp
	userApp
}

type sessionApp interface {
	CreateSession(ctx context.Context, code01 string, code02 string, stepsAmount int) (string, error)
	GetSession(ctx context.Context, sessionId string, userId string) (model.Session, error)

	MakeQuestion(ctx context.Context, sessionId string, userId string, stepIdx int, hiddenQuestion string, openQuestion string) error
	MakeAnswer(ctx context.Context, sessionId string, userId string, stepIdx int, answer string) error

	GetQuestion(ctx context.Context, sessionId string, userId string, stepIdx int) (string, error)
	GetResults(ctx context.Context, sessionId string, userId string) (model.Session, error)
}

func newSessionApp(invites inviterepo.InviteRepo) sessionApp {
	return &sessionAppImpl{
		mu:       &sync.Mutex{},
		sessions: make(map[string]*model.Session),
		invites:  invites,
	}
}

type userApp interface {
	RegisterUser(ctx context.Context, name string, password string) (model.User, error)
	GetUserInviteCode(ctx context.Context, name string, token string) (string, error)
	GetUserSecretCode(ctx context.Context, name string, password string) (string, error)
}

func newUserApp(users userrepo.UserRepo, invites inviterepo.InviteRepo, passwordSalt string) userApp {
	return &userAppImpl{
		users:        users,
		invites:      invites,
		passwordSalt: passwordSalt,
	}
}
