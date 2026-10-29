package sessionrepo

import (
	"context"
	"time"
	"zvrtsh/internal/model"
	"zvrtsh/internal/repo/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionRepo interface {
	CreateSession(ctx context.Context, session model.Session) error
	GetSession(ctx context.Context, id string) (model.Session, error)

	// ListSessions returns sessions of the user, the most recently active first.
	ListSessions(ctx context.Context, userName string) ([]model.Session, error)

	// UpdateSession applies update to the latest state of the session and saves
	// the result with the next version. Updates of one session never run at the
	// same time. If update returns an error, nothing is saved.
	UpdateSession(ctx context.Context, id string, update func(*model.Session) error) (model.Session, error)

	// DeleteSessionsBefore removes sessions without any action since t.
	DeleteSessionsBefore(ctx context.Context, t time.Time) error
}

func New(pool *pgxpool.Pool) SessionRepo {
	return postgres.NewSessionRepo(pool)
}
