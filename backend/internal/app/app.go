package app

import (
	"context"
	"log"
	"time"
	sessionrepo "zvrtsh/internal/repo/session_repo"
	userrepo "zvrtsh/internal/repo/user_repo"
)

// purgeInterval is how often expired sessions are removed.
const purgeInterval = time.Minute

type Config struct {
	// InviteTTL is how long the invite code of a session in the lobby can be used.
	InviteTTL time.Duration

	// SessionTTL is how long a session is stored after the last action in it.
	SessionTTL time.Duration
}

// New creates the application. Background cleanup of expired sessions stops
// when ctx is done.
func New(ctx context.Context, cfg Config, users userrepo.UserRepo, sessions sessionrepo.SessionRepo) App {
	a := &app{
		sessionAppImpl: newSessionApp(sessions, cfg.InviteTTL, cfg.SessionTTL, now),
		userAppImpl:    newUserApp(users),
	}
	go a.runPurge(ctx)
	return a
}

type app struct {
	*sessionAppImpl
	*userAppImpl
}

// now is truncated to microseconds, the precision of Postgres, so a stored
// time is equal to the one that was saved.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func (a *app) runPurge(ctx context.Context) {
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.purgeExpired(ctx); err != nil {
				log.Printf("cannot remove expired sessions: %v", err)
			}
		}
	}
}
