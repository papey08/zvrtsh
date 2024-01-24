package app

import (
	"context"
	"sync"
	"zvrtsh/internal/model"
)

type App interface {
	CreateSession(ctx context.Context, id01 uint, id02 uint, stepsAmount int) (uint, error)

	MakeQuestion(ctx context.Context, sessionId uint, userId uint, stepIdx int, hiddenQuestion string, openQuestion string) error
	MakeAnswer(ctx context.Context, sessionId uint, userId uint, stepIdx int, answer string) error

	GetQuestion(ctx context.Context, sessionId uint, userId uint, stepIdx int) (string, error)

	GetResults(ctx context.Context, sessionId uint, userId uint) (model.Session, error)
}

func New() App {
	return &appInterfaceImpl{
		mu:         &sync.Mutex{},
		sessions:   make(map[uint]*model.Session),
		nextFreeId: 0,
	}
}
