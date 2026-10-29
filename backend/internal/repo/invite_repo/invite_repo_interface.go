package inviterepo

import (
	"context"
	"time"
	"zvrtsh/internal/model"
	"zvrtsh/internal/repo/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

type InviteRepo interface {
	// CreateInvite stores a new invite of the user and drops their previous one.
	CreateInvite(ctx context.Context, invite model.Invite) error

	// TakeInvite removes the invite and returns it, so an invite can be taken
	// only once. Checking the expiration is up to the caller.
	TakeInvite(ctx context.Context, code string) (model.Invite, error)

	// DeleteInvite removes the invite of the user; no invite is not an error.
	DeleteInvite(ctx context.Context, userName string) error

	DeleteExpiredInvites(ctx context.Context, now time.Time) error
}

func New(pool *pgxpool.Pool) InviteRepo {
	return postgres.NewInviteRepo(pool)
}
