package app

import (
	"context"
	"log"
	"time"
	inviterepo "zvrtsh/internal/repo/invite_repo"
	sessionrepo "zvrtsh/internal/repo/session_repo"
	userrepo "zvrtsh/internal/repo/user_repo"
)

// purgeInterval is how often expired invites and sessions are removed.
const purgeInterval = time.Minute

type Config struct {
	// InviteTTL is how long an invite code can be used after its creation.
	InviteTTL time.Duration

	// SessionTTL is how long a session is stored after the last action in it.
	SessionTTL time.Duration
}

// New creates the application. Background cleanup of expired invites and
// sessions stops when ctx is done.
func New(ctx context.Context, cfg Config, users userrepo.UserRepo, invites inviterepo.InviteRepo, sessions sessionrepo.SessionRepo) App {
	a := &app{
		sessionAppImpl: newSessionApp(sessions, invites, cfg.SessionTTL, now),
		userAppImpl:    newUserApp(users, invites, cfg.InviteTTL, now),
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
			if err := a.purgeExpiredInvites(ctx); err != nil {
				log.Printf("cannot remove expired invites: %v", err)
			}
			if err := a.purgeExpired(ctx); err != nil {
				log.Printf("cannot remove expired sessions: %v", err)
			}
		}
	}
}
