package app

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
	inviterepo "zvrtsh/internal/repo/invite_repo"
)

type sessionAppImpl struct {
	mu         *sync.Mutex
	sessions   map[uint64]*model.Session
	nextFreeId uint64
	invites    inviterepo.InviteRepo
}

func (a *sessionAppImpl) CreateSession(ctx context.Context, code01 string, code02 string, stepsAmount int) (uint64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	id01, err := a.invites.GetId(ctx, code01)
	if err != nil {
		return 0, err
	}

	id02, err := a.invites.GetId(ctx, code02)
	if err != nil {
		return 0, err
	}

	a.nextFreeId++
	if a.nextFreeId == 0 {
		return 0, model.ErrIdOverflow
	}

	if stepsAmount <= 0 {
		return 0, model.ErrValidationError
	}

	a.sessions[a.nextFreeId] = &model.Session{
		Id: a.nextFreeId,
		Sheets: [2]model.Sheet{
			{
				OwnerId: id01,
				Steps:   make([]model.Step, stepsAmount),
			},
			{
				OwnerId: id02,
				Steps:   make([]model.Step, stepsAmount),
			},
		},
	}

	return a.nextFreeId, nil
}

func (a *sessionAppImpl) MakeQuestion(_ context.Context, sessionId uint64, userId uint64, stepIdx int, hiddenQuestion string, openQuestion string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, ok := a.sessions[sessionId]
	if !ok {
		return model.ErrNoSession
	}

	if stepIdx < 0 || stepIdx >= len(session.Sheets[0].Steps) || hiddenQuestion == "" || openQuestion == "" {
		return model.ErrValidationError
	}

	var sheet model.Sheet
	if stepIdx%2 == 0 { // в этом случае выдаём игроку его лист
		if session.Sheets[0].OwnerId == userId {
			sheet = session.Sheets[0]
		} else if session.Sheets[1].OwnerId == userId {
			sheet = session.Sheets[1]
		} else {
			return model.ErrUserForbidden
		}
	} else { // иначе выдаём противоположный лист
		if session.Sheets[0].OwnerId == userId {
			sheet = session.Sheets[1]
		} else if session.Sheets[1].OwnerId == userId {
			sheet = session.Sheets[0]
		} else {
			return model.ErrUserForbidden
		}
	}

	access := (stepIdx == 0 && sheet.Steps[stepIdx].Status() == model.StatusWaiting) ||
		(stepIdx > 0 &&
			sheet.Steps[stepIdx].Status() == model.StatusWaiting && // можем записать вопросы на следующий шаг, только если есть ответ на предыдущий
			sheet.Steps[stepIdx-1].Status() == model.StatusAnswered)
	if !access {
		return model.ErrStepForbidden
	}

	sheet.Steps[stepIdx].HiddenQuestion = hiddenQuestion
	sheet.Steps[stepIdx].OpenQuestion = openQuestion
	return nil
}

func (a *sessionAppImpl) MakeAnswer(ctx context.Context, sessionId uint64, userId uint64, stepIdx int, answer string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, ok := a.sessions[sessionId]
	if !ok {
		return model.ErrNoSession
	}

	if stepIdx < 0 || stepIdx >= len(session.Sheets[0].Steps) || answer == "" {
		return model.ErrValidationError
	}

	var sheet model.Sheet
	if stepIdx%2 != 0 { // в этом случае выдаём игроку его лист
		if session.Sheets[0].OwnerId == userId {
			sheet = session.Sheets[0]
		} else if session.Sheets[1].OwnerId == userId {
			sheet = session.Sheets[1]
		} else {
			return model.ErrUserForbidden
		}
	} else { // иначе выдаём противоположный лист
		if session.Sheets[0].OwnerId == userId {
			sheet = session.Sheets[1]
		} else if session.Sheets[1].OwnerId == userId {
			sheet = session.Sheets[0]
		} else {
			return model.ErrUserForbidden
		}
	}

	access := sheet.Steps[stepIdx].Status() == model.StatusAnswering
	if !access {
		return model.ErrStepForbidden
	}

	sheet.Steps[stepIdx].Answer = answer
	return nil
}

func (a *sessionAppImpl) GetQuestion(_ context.Context, sessionId uint64, userId uint64, stepIdx int) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, ok := a.sessions[sessionId]
	if !ok {
		return "", model.ErrNoSession
	}

	if stepIdx < 0 || stepIdx >= len(session.Sheets[0].Steps) {
		return "", model.ErrValidationError
	}

	var sheet model.Sheet
	if stepIdx%2 != 0 { // в этом случае выдаём игроку его лист
		if session.Sheets[0].OwnerId == userId {
			sheet = session.Sheets[0]
		} else if session.Sheets[1].OwnerId == userId {
			sheet = session.Sheets[1]
		} else {
			return "", model.ErrUserForbidden
		}
	} else { // иначе выдаём противоположный лист
		if session.Sheets[0].OwnerId == userId {
			sheet = session.Sheets[1]
		} else if session.Sheets[1].OwnerId == userId {
			sheet = session.Sheets[0]
		} else {
			return "", model.ErrUserForbidden
		}
	}

	access := sheet.Steps[stepIdx].Status() == model.StatusAnswering
	if !access {
		return "", model.ErrStepForbidden
	}

	return sheet.Steps[stepIdx].OpenQuestion, nil
}

func (a *sessionAppImpl) GetResults(ctx context.Context, sessionId uint64, userId uint64) (model.Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	session, ok := a.sessions[sessionId]
	if !ok {
		return model.Session{}, model.ErrNoSession
	}

	if session.Sheets[0].OwnerId != userId && session.Sheets[1].OwnerId != userId {
		return model.Session{}, model.ErrUserForbidden
	}

	if session.Status() == model.StatusDone {
		return *session, nil
	} else {
		return model.Session{}, model.ErrSessionUndone
	}
}
