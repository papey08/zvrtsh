package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"
	"zvrtsh/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	createSessionQuery = `
		INSERT INTO "sessions" ("id", "steps_amount", "started", "invite_code", "invite_expires_at", "version", "created_at", "updated_at")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8);`

	sessionColumns = `"id", "steps_amount", "started", "invite_code", "invite_expires_at", "version", "created_at", "updated_at"`

	selectSessionQuery = `
		SELECT ` + sessionColumns + `
		FROM "sessions"
		WHERE "id" = $1`

	listSessionsQuery = `
		SELECT ` + sessionColumns + `
		FROM "sessions"
		WHERE "id" IN (SELECT "session_id" FROM "session_players" WHERE "user_name" = $1)
		ORDER BY "updated_at" DESC;`

	findByInviteQuery = `
		SELECT "id"
		FROM "sessions"
		WHERE "invite_code" = $1;`

	selectPlayersQuery = `
		SELECT "session_id", "idx", "user_name", "finish_vote"
		FROM "session_players"
		WHERE "session_id" = ANY($1::uuid[])
		ORDER BY "session_id", "idx";`

	upsertPlayerQuery = `
		INSERT INTO "session_players" ("session_id", "idx", "user_name", "finish_vote")
		VALUES ($1, $2, $3, $4)
		ON CONFLICT ("session_id", "idx") DO UPDATE
		SET "user_name" = EXCLUDED."user_name",
		    "finish_vote" = EXCLUDED."finish_vote";`

	selectStepsQuery = `
		SELECT "session_id", "sheet", "step", "asker", "answerer", "hidden_question", "open_question", "answer"
		FROM "steps"
		WHERE "session_id" = ANY($1::uuid[]);`

	upsertStepQuery = `
		INSERT INTO "steps" ("session_id", "sheet", "step", "asker", "answerer", "hidden_question", "open_question", "answer")
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT ("session_id", "sheet", "step") DO UPDATE
		SET "asker" = EXCLUDED."asker",
		    "answerer" = EXCLUDED."answerer",
		    "hidden_question" = EXCLUDED."hidden_question",
		    "open_question" = EXCLUDED."open_question",
		    "answer" = EXCLUDED."answer";`

	deleteStepsFromQuery = `
		DELETE FROM "steps"
		WHERE "session_id" = $1 AND "step" >= $2;`

	updateSessionQuery = `
		UPDATE "sessions"
		SET "version" = $2, "updated_at" = $3, "steps_amount" = $4, "started" = $5, "invite_code" = $6, "invite_expires_at" = $7
		WHERE "id" = $1;`

	deleteSessionQuery = `
		DELETE FROM "sessions"
		WHERE "id" = $1;`

	deleteSessionsQuery = `
		DELETE FROM "sessions"
		WHERE "updated_at" < $1;`
)

type SessionRepo struct {
	pool *pgxpool.Pool
}

func NewSessionRepo(pool *pgxpool.Pool) *SessionRepo {
	return &SessionRepo{pool: pool}
}

// CreateSession stores a new session in the lobby with its players.
func (r *SessionRepo) CreateSession(ctx context.Context, s model.Session) error {
	code, expiresAt := inviteColumns(s.Invite)
	batch := &pgx.Batch{}
	batch.Queue(createSessionQuery, s.Id, s.StepsAmount, s.Started, code, expiresAt, s.Version, s.CreatedAt, s.LastActive)
	for i, p := range s.Players {
		batch.Queue(upsertPlayerQuery, s.Id, i, p.Name, p.FinishVote)
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return dbError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
		return dbError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return dbError(err)
	}
	return nil
}

func (r *SessionRepo) GetSession(ctx context.Context, id string) (model.Session, error) {
	return r.get(ctx, r.pool, id, false)
}

func (r *SessionRepo) FindByInvite(ctx context.Context, code string) (string, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, findByInviteQuery, code).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", model.ErrNoInviteCode
	} else if err != nil {
		return "", dbError(err)
	}
	return id.String(), nil
}

// ListSessions returns sessions of the user, the most recently active first.
func (r *SessionRepo) ListSessions(ctx context.Context, userName string) ([]model.Session, error) {
	rows, err := r.pool.Query(ctx, listSessionsQuery, userName)
	if err != nil {
		return nil, dbError(err)
	}
	sessions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*model.Session, error) {
		s, err := scanSession(row)
		return &s, err
	})
	if err != nil {
		return nil, dbError(err)
	}

	if err = loadDetails(ctx, r.pool, sessions); err != nil {
		return nil, err
	}
	result := make([]model.Session, len(sessions))
	for i, s := range sessions {
		result[i] = *s
	}
	return result, nil
}

// UpdateSession locks the session, applies update to it and saves the result
// with the next version. Concurrent updates of a session wait for each other,
// so update always sees the latest state. If update fails, nothing is saved.
func (r *SessionRepo) UpdateSession(ctx context.Context, id string, update func(*model.Session) error) (model.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.Session{}, dbError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	session, err := r.get(ctx, tx, id, true)
	if err != nil {
		return model.Session{}, err
	}
	before := session.Clone()
	if err = update(&session); err != nil {
		return model.Session{}, err
	}
	session.Version = before.Version + 1

	// only the changed players and steps are written; players never leave
	batch := &pgx.Batch{}
	for i, p := range session.Players {
		if i >= len(before.Players) || p != before.Players[i] {
			batch.Queue(upsertPlayerQuery, id, i, p.Name, p.FinishVote)
		}
	}
	for sheet := range session.Sheets {
		for step, after := range session.Sheets[sheet].Steps {
			if sheet < len(before.Sheets) && after == before.Sheets[sheet].Steps[step] {
				continue
			}
			batch.Queue(upsertStepQuery, id, sheet, step, after.Asker, after.Answerer,
				nullIfEmpty(after.HiddenQuestion), nullIfEmpty(after.OpenQuestion), nullIfEmpty(after.Answer))
		}
	}
	if session.StepsAmount < before.StepsAmount {
		batch.Queue(deleteStepsFromQuery, id, session.StepsAmount)
	}
	code, expiresAt := inviteColumns(session.Invite)
	batch.Queue(updateSessionQuery, id, session.Version, session.LastActive, session.StepsAmount, session.Started, code, expiresAt)

	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
		return model.Session{}, dbError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return model.Session{}, dbError(err)
	}
	return session, nil
}

// DeleteSession locks the session, lets check decide and removes it with its players and steps.
func (r *SessionRepo) DeleteSession(ctx context.Context, id string, check func(model.Session) error) (model.Session, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return model.Session{}, dbError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	session, err := r.get(ctx, tx, id, true)
	if err != nil {
		return model.Session{}, err
	}
	if err = check(session); err != nil {
		return model.Session{}, err
	}
	if _, err = tx.Exec(ctx, deleteSessionQuery, id); err != nil {
		return model.Session{}, dbError(err)
	}
	if err = tx.Commit(ctx); err != nil {
		return model.Session{}, dbError(err)
	}
	return session, nil
}

// DeleteSessionsBefore removes sessions without any action since t, with their steps.
func (r *SessionRepo) DeleteSessionsBefore(ctx context.Context, t time.Time) error {
	if _, err := r.pool.Exec(ctx, deleteSessionsQuery, t); err != nil {
		return dbError(err)
	}
	return nil
}

func (r *SessionRepo) get(ctx context.Context, q querier, id string, forUpdate bool) (model.Session, error) {
	// the column is UUID, any other string would be a query error instead of "not found"
	if uuid.Validate(id) != nil {
		return model.Session{}, model.ErrNoSession
	}

	query := selectSessionQuery
	if forUpdate {
		query += " FOR UPDATE"
	}
	session, err := scanSession(q.QueryRow(ctx, query, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Session{}, model.ErrNoSession
	} else if err != nil {
		return model.Session{}, dbError(err)
	}

	if err = loadDetails(ctx, q, []*model.Session{&session}); err != nil {
		return model.Session{}, err
	}
	return session, nil
}

// scanSession reads a row of the sessions table, players and steps stay empty.
func scanSession(row pgx.Row) (model.Session, error) {
	var (
		s         model.Session
		id        uuid.UUID
		code      *string
		expiresAt *time.Time
	)
	if err := row.Scan(&id, &s.StepsAmount, &s.Started, &code, &expiresAt, &s.Version, &s.CreatedAt, &s.LastActive); err != nil {
		return model.Session{}, err
	}

	s.Id = id.String()
	s.CreatedAt = s.CreatedAt.UTC()
	s.LastActive = s.LastActive.UTC()
	if code != nil && expiresAt != nil {
		s.Invite = &model.Invite{Code: *code, ExpiresAt: expiresAt.UTC()}
	}
	return s, nil
}

// loadDetails fills the players and the steps of the sessions, with a query for each.
func loadDetails(ctx context.Context, q querier, sessions []*model.Session) error {
	if len(sessions) == 0 {
		return nil
	}
	byId := make(map[string]*model.Session, len(sessions))
	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		byId[s.Id] = s
		ids = append(ids, s.Id)
	}

	if err := loadPlayers(ctx, q, ids, byId); err != nil {
		return err
	}
	for _, s := range sessions {
		if s.Started {
			s.Sheets = make([]model.Sheet, len(s.Players))
			for i := range s.Sheets {
				s.Sheets[i].Steps = make([]model.Step, s.StepsAmount)
			}
		}
	}
	return loadSteps(ctx, q, ids, byId)
}

func loadPlayers(ctx context.Context, q querier, ids []string, byId map[string]*model.Session) error {
	rows, err := q.Query(ctx, selectPlayersQuery, ids)
	if err != nil {
		return dbError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			sessionId uuid.UUID
			idx       int
			player    model.Player
		)
		if err = rows.Scan(&sessionId, &idx, &player.Name, &player.FinishVote); err != nil {
			return dbError(err)
		}
		s := byId[sessionId.String()]
		if s == nil || idx != len(s.Players) {
			return dbError(fmt.Errorf("player %d does not fit session %s", idx, sessionId))
		}
		s.Players = append(s.Players, player)
	}
	if err = rows.Err(); err != nil {
		return dbError(err)
	}
	return nil
}

func loadSteps(ctx context.Context, q querier, ids []string, byId map[string]*model.Session) error {
	rows, err := q.Query(ctx, selectStepsQuery, ids)
	if err != nil {
		return dbError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			sessionId            uuid.UUID
			sheet, step          int
			value                model.Step
			hidden, open, answer *string
		)
		if err = rows.Scan(&sessionId, &sheet, &step, &value.Asker, &value.Answerer, &hidden, &open, &answer); err != nil {
			return dbError(err)
		}
		s := byId[sessionId.String()]
		if s == nil || sheet < 0 || sheet >= len(s.Sheets) || step < 0 || step >= s.StepsAmount {
			return dbError(fmt.Errorf("step %d of sheet %d does not fit session %s", step, sheet, sessionId))
		}
		value.HiddenQuestion, value.OpenQuestion, value.Answer = deref(hidden), deref(open), deref(answer)
		s.Sheets[sheet].Steps[step] = value
	}
	if err = rows.Err(); err != nil {
		return dbError(err)
	}
	return nil
}

func inviteColumns(invite *model.Invite) (*string, *time.Time) {
	if invite == nil {
		return nil, nil
	}
	return &invite.Code, &invite.ExpiresAt
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
