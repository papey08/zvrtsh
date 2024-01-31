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
	CreateSession(ctx context.Context, code01 string, code02 string, stepsAmount int) (uint64, error)
	GetSession(ctx context.Context, sessionId uint64, userId uint64) (model.Session, error)

	MakeQuestion(ctx context.Context, sessionId uint64, userId uint64, stepIdx int, hiddenQuestion string, openQuestion string) error
	MakeAnswer(ctx context.Context, sessionId uint64, userId uint64, stepIdx int, answer string) error

	GetQuestion(ctx context.Context, sessionId uint64, userId uint64, stepIdx int) (string, error)
	GetResults(ctx context.Context, sessionId uint64, userId uint64) (model.Session, error)
}

func newSessionApp(invites inviterepo.InviteRepo) sessionApp {
	return &sessionAppImpl{
		mu:       &sync.Mutex{},
		sessions: make(map[uint64]*model.Session),
		invites:  invites,
	}
}

type userApp interface {
	RegisterUser(ctx context.Context, name string, password string) (model.User, error)
	GetUserInviteCode(ctx context.Context, name string, token uint64) (string, error)
	GetUserSecretCode(ctx context.Context, name string, password string) (uint64, error)
}

func newUserApp(invites inviterepo.InviteRepo) userApp {
	return &userAppImpl{
		users:   userrepo.New(),
		invites: invites,
	}
}
