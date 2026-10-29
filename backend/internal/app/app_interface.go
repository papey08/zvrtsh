package app

import (
	"context"
	"zvrtsh/internal/model"
)

// App describes the whole business logic. userName is always the name of
// the authenticated user.
type App interface {
	sessionApp
	userApp
}

type sessionApp interface {
	CreateSession(ctx context.Context, userName string, inviteCode string) (model.Session, error)
	ListSessions(ctx context.Context, userName string) ([]model.Session, error)
	GetSession(ctx context.Context, sessionId string, userName string) (model.Session, error)

	MakeQuestion(ctx context.Context, sessionId string, userName string, stepIdx int, hiddenQuestion string, openQuestion string) error
	MakeAnswer(ctx context.Context, sessionId string, userName string, stepIdx int, answer string) error

	GetOpponentQuestion(ctx context.Context, sessionId string, userName string, stepIdx int) (string, error)
	GetResults(ctx context.Context, sessionId string, userName string) (model.Session, error)

	// VoteFinish offers (vote) or stops offering to finish the session after the current step.
	VoteFinish(ctx context.Context, sessionId string, userName string, vote bool) (model.Session, error)

	SubscribeSessions(ctx context.Context, userName string) (sessions <-chan model.Session, unsubscribe func())
}

type userApp interface {
	RegisterUser(ctx context.Context, name string, password string) (model.User, error)
	CreateToken(ctx context.Context, name string, password string) (string, error)
	Authenticate(ctx context.Context, token string) (string, error)
	CreateInvite(ctx context.Context, userName string, stepsAmount int) (model.Invite, error)
	DeleteInvite(ctx context.Context, userName string) error
}
