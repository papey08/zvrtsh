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
		INSERT INTO "sessions" ("id", "player_1", "player_2", "steps_amount", "version", "created_at", "updated_at")
		VALUES ($1, $2, $3, $4, $5, $6, $7);`

	selectSessionQuery = `
		SELECT "id", "player_1", "player_2", "steps_amount", "version", "created_at", "updated_at", "finish_vote_1", "finish_vote_2"
		FROM "sessions"
		WHERE "id" = $1`

	listSessionsQuery = `
		SELECT "id", "player_1", "player_2", "steps_amount", "version", "created_at", "updated_at", "finish_vote_1", "finish_vote_2"
		FROM "sessions"
		WHERE "player_1" = $1 OR "player_2" = $1
		ORDER BY "updated_at" DESC;`

	selectStepsQuery = `
		SELECT "session_id", "sheet", "step", "hidden_question", "open_question", "answer"
		FROM "steps"
		WHERE "session_id" = ANY($1::uuid[]);`

	upsertStepQuery = `
		INSERT INTO "steps" ("session_id", "sheet", "step", "hidden_question", "open_question", "answer")
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT ("session_id", "sheet", "step") DO UPDATE
		SET "hidden_question" = EXCLUDED."hidden_question",
		    "open_question" = EXCLUDED."open_question",
		    "answer" = EXCLUDED."answer";`

	deleteStepQuery = `
		DELETE FROM "steps"
		WHERE "session_id" = $1 AND "sheet" = $2 AND "step" = $3;`

	updateSessionQuery = `
		UPDATE "sessions"
		SET "version" = $2, "updated_at" = $3, "steps_amount" = $4, "finish_vote_1" = $5, "finish_vote_2" = $6
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

// CreateSession stores a new session. Its steps must be empty.
func (r *SessionRepo) CreateSession(ctx context.Context, s model.Session) error {
	_, err := r.pool.Exec(ctx, createSessionQuery, s.Id, s.Sheets[0].Owner, s.Sheets[1].Owner,
		len(s.Sheets[0].Steps), s.Version, s.CreatedAt, s.LastActive)
	if err != nil {
		return dbError(err)
	}
	return nil
}

func (r *SessionRepo) GetSession(ctx context.Context, id string) (model.Session, error) {
	return r.get(ctx, r.pool, id, false)
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

	if err = loadSteps(ctx, r.pool, sessions); err != nil {
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

	// only the changed steps are written
	batch := &pgx.Batch{}
	for sheet := range session.Sheets {
		// a shortened session only drops empty steps, there is nothing to delete
		for step, after := range session.Sheets[sheet].Steps {
			if after == before.Sheets[sheet].Steps[step] {
				continue
			}
			if after.Status() == model.StepWaiting {
				batch.Queue(deleteStepQuery, id, sheet, step)
			} else {
				batch.Queue(upsertStepQuery, id, sheet, step, after.HiddenQuestion, after.OpenQuestion, nullIfEmpty(after.Answer))
			}
		}
	}
	batch.Queue(updateSessionQuery, id, session.Version, session.LastActive,
		len(session.Sheets[0].Steps), session.FinishVotes[0], session.FinishVotes[1])

	if err = tx.SendBatch(ctx, batch).Close(); err != nil {
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

	if err = loadSteps(ctx, q, []*model.Session{&session}); err != nil {
		return model.Session{}, err
	}
	return session, nil
}

// scanSession reads a row of the sessions table, the steps stay empty.
func scanSession(row pgx.Row) (model.Session, error) {
	var (
		s           model.Session
		id          uuid.UUID
		players     [2]string
		stepsAmount int
	)
	if err := row.Scan(&id, &players[0], &players[1], &stepsAmount, &s.Version, &s.CreatedAt, &s.LastActive,
		&s.FinishVotes[0], &s.FinishVotes[1]); err != nil {
		return model.Session{}, err
	}

	s.Id = id.String()
	s.CreatedAt = s.CreatedAt.UTC()
	s.LastActive = s.LastActive.UTC()
	for i, owner := range players {
		s.Sheets[i] = model.Sheet{Owner: owner, Steps: make([]model.Step, stepsAmount)}
	}
	return s, nil
}

// loadSteps fills the steps of the sessions with one query.
func loadSteps(ctx context.Context, q querier, sessions []*model.Session) error {
	if len(sessions) == 0 {
		return nil
	}
	byId := make(map[string]*model.Session, len(sessions))
	ids := make([]string, 0, len(sessions))
	for _, s := range sessions {
		byId[s.Id] = s
		ids = append(ids, s.Id)
	}

	rows, err := q.Query(ctx, selectStepsQuery, ids)
	if err != nil {
		return dbError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			sessionId    uuid.UUID
			sheet, step  int
			hidden, open string
			answer       *string
		)
		if err = rows.Scan(&sessionId, &sheet, &step, &hidden, &open, &answer); err != nil {
			return dbError(err)
		}
		s := byId[sessionId.String()]
		if s == nil || sheet < 0 || sheet > 1 || step < 0 || step >= len(s.Sheets[sheet].Steps) {
			return dbError(fmt.Errorf("step %d of sheet %d does not fit session %s", step, sheet, sessionId))
		}
		s.Sheets[sheet].Steps[step] = model.Step{HiddenQuestion: hidden, OpenQuestion: open}
		if answer != nil {
			s.Sheets[sheet].Steps[step].Answer = *answer
		}
	}
	if err = rows.Err(); err != nil {
		return dbError(err)
	}
	return nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
