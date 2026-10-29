package app

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
	"zvrtsh/internal/model"
	inviterepo "zvrtsh/internal/repo/invite_repo"
	sessionrepo "zvrtsh/internal/repo/session_repo"

	"github.com/google/uuid"
)

const (
	maxStepsAmount = 50
	maxTextLen     = 500 // symbols in a question or an answer
)

type sessionAppImpl struct {
	sessions sessionrepo.SessionRepo
	invites  inviterepo.InviteRepo
	hub      *hub
	ttl      time.Duration
	now      func() time.Time
}

func newSessionApp(sessions sessionrepo.SessionRepo, invites inviterepo.InviteRepo, ttl time.Duration, now func() time.Time) *sessionAppImpl {
	return &sessionAppImpl{
		sessions: sessions,
		invites:  invites,
		hub:      newHub(),
		ttl:      ttl,
		now:      now,
	}
}

// CreateSession starts a session with the owner of the invite, the number of
// steps is the one chosen by the owner.
func (a *sessionAppImpl) CreateSession(ctx context.Context, userName string, inviteCode string) (model.Session, error) {
	invite, err := a.invites.TakeInvite(ctx, inviteCode)
	if err != nil {
		return model.Session{}, err
	}
	now := a.now()
	if !now.Before(invite.ExpiresAt) {
		return model.Session{}, model.ErrNoInviteCode
	}
	if invite.UserName == userName {
		return model.Session{}, fmt.Errorf("%w: cannot start a session with yourself", model.ErrValidation)
	}

	session := model.Session{
		Id: uuid.New().String(),
		Sheets: [2]model.Sheet{
			{Owner: userName, Steps: make([]model.Step, invite.StepsAmount)},
			{Owner: invite.UserName, Steps: make([]model.Step, invite.StepsAmount)},
		},
		CreatedAt:  now,
		LastActive: now,
		Version:    1,
	}
	if err = a.sessions.CreateSession(ctx, session); err != nil {
		return model.Session{}, err
	}
	a.hub.publish(session.Clone())
	return session, nil
}

// SubscribeSessions returns a channel with every created or changed session of
// the user. The channel is closed when the subscriber falls behind or after
// unsubscribe is called. Updates of a session may come out of order, the
// newest one has the greatest version.
func (a *sessionAppImpl) SubscribeSessions(_ context.Context, userName string) (<-chan model.Session, func()) {
	ch := a.hub.subscribe(userName)
	return ch, func() { a.hub.unsubscribe(userName, ch) }
}

// ListSessions returns sessions of the user, the most recently active first.
func (a *sessionAppImpl) ListSessions(ctx context.Context, userName string) ([]model.Session, error) {
	return a.sessions.ListSessions(ctx, userName)
}

func (a *sessionAppImpl) GetSession(ctx context.Context, sessionId string, userName string) (model.Session, error) {
	session, _, err := a.find(ctx, sessionId, userName)
	return session, err
}

// MakeQuestion writes a question of the player. On even steps it goes to the
// player's own sheet, on odd steps to the opponent's one.
func (a *sessionAppImpl) MakeQuestion(ctx context.Context, sessionId string, userName string, stepIdx int, hiddenQuestion string, openQuestion string) error {
	if !isValidText(hiddenQuestion) || !isValidText(openQuestion) {
		return model.ErrValidation
	}

	return a.update(ctx, sessionId, userName, stepIdx, func(session *model.Session, me int) error {
		sheet := &session.Sheets[model.QuestionSheetIdx(stepIdx, me)]
		// a question can be written only if the previous step of the sheet is answered
		if sheet.Steps[stepIdx].Status() != model.StepWaiting ||
			stepIdx > 0 && sheet.Steps[stepIdx-1].Status() != model.StepAnswered {
			return model.ErrStepUnavailable
		}

		sheet.Steps[stepIdx].HiddenQuestion = hiddenQuestion
		sheet.Steps[stepIdx].OpenQuestion = openQuestion
		return nil
	})
}

// MakeAnswer answers the opponent's question of the step.
func (a *sessionAppImpl) MakeAnswer(ctx context.Context, sessionId string, userName string, stepIdx int, answer string) error {
	if !isValidText(answer) {
		return model.ErrValidation
	}

	return a.update(ctx, sessionId, userName, stepIdx, func(session *model.Session, me int) error {
		step := &session.Sheets[model.AnswerSheetIdx(stepIdx, me)].Steps[stepIdx]
		if step.Status() != model.StepAnswering {
			return model.ErrStepUnavailable
		}

		step.Answer = answer
		return nil
	})
}

// update changes the step of the session on behalf of the player and notifies both players.
func (a *sessionAppImpl) update(ctx context.Context, sessionId string, userName string, stepIdx int, change func(session *model.Session, me int) error) error {
	updated, err := a.sessions.UpdateSession(ctx, sessionId, func(session *model.Session) error {
		me, ok := session.PlayerIdx(userName)
		if !ok {
			return model.ErrNotParticipant
		}
		if err := checkStepIdx(session, stepIdx); err != nil {
			return err
		}
		if err := change(session, me); err != nil {
			return err
		}
		session.LastActive = a.now()
		return nil
	})
	if err != nil {
		return err
	}

	a.hub.publish(updated)
	return nil
}

// VoteFinish sets the player's offer to finish the session after the current
// step. When both players agree, the session is shortened to the current step
// and the votes are reset.
func (a *sessionAppImpl) VoteFinish(ctx context.Context, sessionId string, userName string, vote bool) (model.Session, error) {
	updated, err := a.sessions.UpdateSession(ctx, sessionId, func(session *model.Session) error {
		me, ok := session.PlayerIdx(userName)
		if !ok {
			return model.ErrNotParticipant
		}
		if session.Status() == model.StatusDone {
			return model.ErrSessionDone
		}

		session.FinishVotes[me] = vote
		if session.FinishVotes[0] && session.FinishVotes[1] {
			session.Shorten(session.CurrentStep() + 1)
			session.FinishVotes = [2]bool{}
		}
		session.LastActive = a.now()
		return nil
	})
	if err != nil {
		return model.Session{}, err
	}

	a.hub.publish(updated)
	return updated, nil
}

// GetOpponentQuestion returns the open part of the question the player has to answer.
func (a *sessionAppImpl) GetOpponentQuestion(ctx context.Context, sessionId string, userName string, stepIdx int) (string, error) {
	session, me, err := a.find(ctx, sessionId, userName)
	if err != nil {
		return "", err
	}
	if err = checkStepIdx(&session, stepIdx); err != nil {
		return "", err
	}

	step := session.Sheets[model.AnswerSheetIdx(stepIdx, me)].Steps[stepIdx]
	if step.Status() == model.StepWaiting {
		return "", model.ErrStepUnavailable
	}
	return step.OpenQuestion, nil
}

func (a *sessionAppImpl) GetResults(ctx context.Context, sessionId string, userName string) (model.Session, error) {
	session, _, err := a.find(ctx, sessionId, userName)
	if err != nil {
		return model.Session{}, err
	}
	if session.Status() != model.StatusDone {
		return model.Session{}, model.ErrSessionUndone
	}
	return session, nil
}

// find returns the session and the index of the user's sheet.
func (a *sessionAppImpl) find(ctx context.Context, sessionId string, userName string) (model.Session, int, error) {
	session, err := a.sessions.GetSession(ctx, sessionId)
	if err != nil {
		return model.Session{}, 0, err
	}
	me, ok := session.PlayerIdx(userName)
	if !ok {
		return model.Session{}, 0, model.ErrNotParticipant
	}
	return session, me, nil
}

func checkStepIdx(session *model.Session, stepIdx int) error {
	if stepIdx < 0 || stepIdx >= len(session.Sheets[0].Steps) {
		return fmt.Errorf("%w: there is no step with index %d", model.ErrValidation, stepIdx)
	}
	return nil
}

func isValidText(s string) bool {
	return strings.TrimSpace(s) != "" && utf8.ValidString(s) && utf8.RuneCountInString(s) <= maxTextLen
}

// purgeExpired removes sessions without any action during the ttl.
func (a *sessionAppImpl) purgeExpired(ctx context.Context) error {
	return a.sessions.DeleteSessionsBefore(ctx, a.now().Add(-a.ttl))
}
