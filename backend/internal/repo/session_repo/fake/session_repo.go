// Package fake contains an in-memory session repository for tests.
package fake

import (
	"context"
	"sort"
	"sync"
	"time"
	"zvrtsh/internal/model"
)

type SessionRepo struct {
	mu       sync.Mutex
	sessions map[string]model.Session
}

func New() *SessionRepo {
	return &SessionRepo{sessions: make(map[string]model.Session)}
}

func (r *SessionRepo) CreateSession(_ context.Context, session model.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessions[session.Id] = session.Clone()
	return nil
}

func (r *SessionRepo) GetSession(_ context.Context, id string) (model.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[id]
	if !ok {
		return model.Session{}, model.ErrNoSession
	}
	return session.Clone(), nil
}

func (r *SessionRepo) FindByInvite(_ context.Context, code string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		if session.Invite != nil && session.Invite.Code == code {
			return id, nil
		}
	}
	return "", model.ErrNoInviteCode
}

func (r *SessionRepo) ListSessions(_ context.Context, userName string) ([]model.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	sessions := make([]model.Session, 0)
	for _, session := range r.sessions {
		if _, ok := session.PlayerIdx(userName); ok {
			sessions = append(sessions, session.Clone())
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActive.After(sessions[j].LastActive)
	})
	return sessions, nil
}

func (r *SessionRepo) UpdateSession(_ context.Context, id string, update func(*model.Session) error) (model.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, ok := r.sessions[id]
	if !ok {
		return model.Session{}, model.ErrNoSession
	}
	session := stored.Clone()
	if err := update(&session); err != nil {
		return model.Session{}, err
	}
	session.Version = stored.Version + 1
	r.sessions[id] = session.Clone()
	return session, nil
}

func (r *SessionRepo) DeleteSession(_ context.Context, id string, check func(model.Session) error) (model.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[id]
	if !ok {
		return model.Session{}, model.ErrNoSession
	}
	if err := check(session.Clone()); err != nil {
		return model.Session{}, err
	}
	delete(r.sessions, id)
	return session, nil
}

func (r *SessionRepo) DeleteSessionsBefore(_ context.Context, t time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, session := range r.sessions {
		if session.LastActive.Before(t) {
			delete(r.sessions, id)
		}
	}
	return nil
}
