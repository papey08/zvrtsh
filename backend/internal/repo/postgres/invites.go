package postgres

import (
	"context"
	"errors"
	"time"
	"zvrtsh/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// a new invite of the user takes the place of the previous one
	createInviteQuery = `
		INSERT INTO "invites" ("code", "user_name", "steps_amount", "created_at", "expires_at")
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT ("user_name") DO UPDATE
		SET "code" = EXCLUDED."code",
		    "steps_amount" = EXCLUDED."steps_amount",
		    "created_at" = EXCLUDED."created_at",
		    "expires_at" = EXCLUDED."expires_at";`

	takeInviteQuery = `
		DELETE FROM "invites"
		WHERE "code" = $1
		RETURNING "code", "user_name", "steps_amount", "created_at", "expires_at";`

	deleteInviteQuery = `
		DELETE FROM "invites"
		WHERE "user_name" = $1;`

	deleteExpiredInvitesQuery = `
		DELETE FROM "invites"
		WHERE "expires_at" <= $1;`
)

type InviteRepo struct {
	pool *pgxpool.Pool
}

func NewInviteRepo(pool *pgxpool.Pool) *InviteRepo {
	return &InviteRepo{pool: pool}
}

func (i *InviteRepo) CreateInvite(ctx context.Context, invite model.Invite) error {
	_, err := i.pool.Exec(ctx, createInviteQuery, invite.Code, invite.UserName, invite.StepsAmount, invite.CreationDate, invite.ExpiresAt)
	if err != nil {
		return dbError(err)
	}
	return nil
}

// TakeInvite deletes the invite in the same statement, so it can be taken only once.
func (i *InviteRepo) TakeInvite(ctx context.Context, code string) (model.Invite, error) {
	var invite model.Invite
	err := i.pool.QueryRow(ctx, takeInviteQuery, code).
		Scan(&invite.Code, &invite.UserName, &invite.StepsAmount, &invite.CreationDate, &invite.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Invite{}, model.ErrNoInviteCode
	} else if err != nil {
		return model.Invite{}, dbError(err)
	}
	invite.CreationDate = invite.CreationDate.UTC()
	invite.ExpiresAt = invite.ExpiresAt.UTC()
	return invite, nil
}

func (i *InviteRepo) DeleteInvite(ctx context.Context, userName string) error {
	if _, err := i.pool.Exec(ctx, deleteInviteQuery, userName); err != nil {
		return dbError(err)
	}
	return nil
}

func (i *InviteRepo) DeleteExpiredInvites(ctx context.Context, now time.Time) error {
	if _, err := i.pool.Exec(ctx, deleteExpiredInvitesQuery, now); err != nil {
		return dbError(err)
	}
	return nil
}
