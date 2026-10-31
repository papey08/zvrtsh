package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
	"unicode/utf8"
	"zvrtsh/internal/model"
	sessionrepo "zvrtsh/internal/repo/session_repo"

	"github.com/google/uuid"
)

const (
	maxStepsAmount = 10
	maxTextLen     = 500 // symbols in a question or an answer
)

type sessionAppImpl struct {
	sessions  sessionrepo.SessionRepo
	hub       *hub
	inviteTTL time.Duration
	ttl       time.Duration
	now       func() time.Time
	newRand   func() *rand.Rand // the source of the deal of a started session
}

func newSessionApp(sessions sessionrepo.SessionRepo, inviteTTL time.Duration, ttl time.Duration, now func() time.Time) *sessionAppImpl {
	return &sessionAppImpl{
		sessions:  sessions,
		hub:       newHub(),
		inviteTTL: inviteTTL,
		ttl:       ttl,
		now:       now,
		newRand: func() *rand.Rand {
			return rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
		},
	}
}

// CreateInvite creates a session in the lobby with the user as the host and
// returns it with the invite. The session the user still waits in as the
// host is cancelled.
func (a *sessionAppImpl) CreateInvite(ctx context.Context, userName string, stepsAmount int) (model.Session, error) {
	if stepsAmount < 1 || stepsAmount > maxStepsAmount {
		return model.Session{}, fmt.Errorf("%w: steps amount must be between 1 and %d", model.ErrValidation, maxStepsAmount)
	}
	if err := a.DeleteInvite(ctx, userName); err != nil {
		return model.Session{}, err
	}

	now := a.now()
	session := model.Session{
		Id:          uuid.New().String(),
		Players:     []model.Player{{Name: userName}},
		StepsAmount: stepsAmount,
		Invite:      &model.Invite{Code: uuid.New().String(), ExpiresAt: now.Add(a.inviteTTL)},
		CreatedAt:   now,
		LastActive:  now,
		Version:     1,
	}
	if err := a.sessions.CreateSession(ctx, session); err != nil {
		return model.Session{}, err
	}
	a.hub.publish(model.SessionEvent{Session: session.Clone()})
	return session, nil
}

// DeleteInvite cancels the sessions the user waits in as the host, if there
// are any. Their players are told about it.
func (a *sessionAppImpl) DeleteInvite(ctx context.Context, userName string) error {
	sessions, err := a.sessions.ListSessions(ctx, userName)
	if err != nil {
		return err
	}
	for _, s := range sessions {
		if s.Started || s.Host() != userName {
			continue
		}
		deleted, err := a.sessions.DeleteSession(ctx, s.Id, func(session model.Session) error {
			if session.Started {
				return model.ErrSessionStarted
			}
			return nil
		})
		switch {
		case err == nil:
			a.hub.publish(model.SessionEvent{Session: deleted, Deleted: true})
		case !isGone(err):
			return err
		}
	}
	return nil
}

// isGone tells that the session has started or disappeared meanwhile, which is
// not an error for cancelling it.
func isGone(err error) bool {
	return errors.Is(err, model.ErrSessionStarted) || errors.Is(err, model.ErrNoSession)
}

// JoinSession adds the user to the players of the session in the lobby with the invite.
func (a *sessionAppImpl) JoinSession(ctx context.Context, userName string, inviteCode string) (model.Session, error) {
	id, err := a.sessions.FindByInvite(ctx, inviteCode)
	if err != nil {
		return model.Session{}, err
	}

	joined := false
	updated, err := a.sessions.UpdateSession(ctx, id, func(session *model.Session) error {
		if _, ok := session.PlayerIdx(userName); ok {
			return errAlreadyJoined
		}
		if session.Invite == nil || session.Invite.Code != inviteCode || !a.now().Before(session.Invite.ExpiresAt) {
			return model.ErrNoInviteCode
		}
		if session.Started {
			return model.ErrSessionStarted
		}
		if len(session.Players) >= model.MaxPlayers {
			return model.ErrSessionFull
		}

		session.Players = append(session.Players, model.Player{Name: userName})
		session.LastActive = a.now()
		joined = true
		return nil
	})
	if errors.Is(err, errAlreadyJoined) {
		// opening the invite link again leads the player back to the session
		return a.sessions.GetSession(ctx, id)
	}
	if err != nil {
		return model.Session{}, err
	}

	if joined {
		a.hub.publish(model.SessionEvent{Session: updated})
	}
	return updated, nil
}

var errAlreadyJoined = errors.New("already joined")

// StartSession starts the session in the lobby with the players who have
// joined it. Only the host can do it.
func (a *sessionAppImpl) StartSession(ctx context.Context, sessionId string, userName string) (model.Session, error) {
	updated, err := a.sessions.UpdateSession(ctx, sessionId, func(session *model.Session) error {
		me, ok := session.PlayerIdx(userName)
		if !ok {
			return model.ErrNotParticipant
		}
		if me != 0 {
			return model.ErrNotHost
		}
		if session.Started {
			return model.ErrSessionStarted
		}
		if len(session.Players) < model.MinPlayers {
			return model.ErrTooFewPlayers
		}

		session.Start(a.newRand())
		session.LastActive = a.now()
		return nil
	})
	if err != nil {
		return model.Session{}, err
	}

	a.hub.publish(model.SessionEvent{Session: updated})
	return updated, nil
}

// SubscribeSessions returns a channel with every created, changed or deleted
// session of the user. The channel is closed when the subscriber falls behind
// or after unsubscribe is called. Updates of a session may come out of order,
// the newest one has the greatest version.
func (a *sessionAppImpl) SubscribeSessions(_ context.Context, userName string) (<-chan model.SessionEvent, func()) {
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

// MakeQuestion writes a question of the player on the sheet they hold.
func (a *sessionAppImpl) MakeQuestion(ctx context.Context, sessionId string, userName string, stepIdx int, hiddenQuestion string, openQuestion string) error {
	if !isValidText(hiddenQuestion) || !isValidText(openQuestion) {
		return model.ErrValidation
	}

	return a.update(ctx, sessionId, userName, stepIdx, func(session *model.Session, me int) error {
		sheet := &session.Sheets[session.QuestionSheetIdx(stepIdx, me)]
		// the sheet stays with the player after the answer: the question is
		// written once the player has answered the previous step on it
		if sheet.Steps[stepIdx].Status() != model.StepWaiting ||
			stepIdx > 0 && sheet.Steps[stepIdx-1].Status() != model.StepAnswered {
			return model.ErrStepUnavailable
		}

		sheet.Steps[stepIdx].HiddenQuestion = hiddenQuestion
		sheet.Steps[stepIdx].OpenQuestion = openQuestion
		return nil
	})
}

// MakeAnswer answers the question of the step on the sheet dealt to the player.
func (a *sessionAppImpl) MakeAnswer(ctx context.Context, sessionId string, userName string, stepIdx int, answer string) error {
	if !isValidText(answer) {
		return model.ErrValidation
	}

	return a.update(ctx, sessionId, userName, stepIdx, func(session *model.Session, me int) error {
		step := &session.Sheets[session.AnswerSheetIdx(stepIdx, me)].Steps[stepIdx]
		// a question can be answered only when everybody has written their question
		if step.Status() != model.StepAnswering || !session.AnswersOpen(stepIdx) {
			return model.ErrStepUnavailable
		}

		step.Answer = answer
		return nil
	})
}

// update changes the step of the session on behalf of the player and notifies all players.
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

	a.hub.publish(model.SessionEvent{Session: updated})
	return nil
}

// VoteFinish sets the player's offer to finish the session after the current
// step. When all players agree, the session is shortened to the current step
// and the votes are reset.
func (a *sessionAppImpl) VoteFinish(ctx context.Context, sessionId string, userName string, vote bool) (model.Session, error) {
	updated, err := a.sessions.UpdateSession(ctx, sessionId, func(session *model.Session) error {
		me, ok := session.PlayerIdx(userName)
		if !ok {
			return model.ErrNotParticipant
		}
		switch session.Status() {
		case model.StatusLobby:
			return model.ErrSessionNotStarted
		case model.StatusDone:
			return model.ErrSessionDone
		}

		session.Players[me].FinishVote = vote
		if session.AllVotedToFinish() {
			session.Shorten(session.CurrentStep() + 1)
			for i := range session.Players {
				session.Players[i].FinishVote = false
			}
		}
		session.LastActive = a.now()
		return nil
	})
	if err != nil {
		return model.Session{}, err
	}

	a.hub.publish(model.SessionEvent{Session: updated})
	return updated, nil
}

// GetQuestionToAnswer returns the open part of the question the player has to answer.
func (a *sessionAppImpl) GetQuestionToAnswer(ctx context.Context, sessionId string, userName string, stepIdx int) (string, error) {
	session, me, err := a.find(ctx, sessionId, userName)
	if err != nil {
		return "", err
	}
	if err = checkStepIdx(&session, stepIdx); err != nil {
		return "", err
	}

	if !session.AnswersOpen(stepIdx) {
		return "", model.ErrStepUnavailable
	}
	return session.Sheets[session.AnswerSheetIdx(stepIdx, me)].Steps[stepIdx].OpenQuestion, nil
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

// find returns the session and the index of the user in it.
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

// checkStepIdx also rejects the steps of a session in the lobby, it has none yet.
func checkStepIdx(session *model.Session, stepIdx int) error {
	if !session.Started {
		return model.ErrSessionNotStarted
	}
	if stepIdx < 0 || stepIdx >= session.StepsAmount {
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
