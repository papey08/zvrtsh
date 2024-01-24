package model

import "errors"

var (
	ErrIdOverflow      = errors.New("no more available ids for new sessions")
	ErrNoSession       = errors.New("there is no session with required id")
	ErrValidationError = errors.New("some of your data is invalid")
	ErrUserForbidden   = errors.New("there is no user with required id in this session")
	ErrStepForbidden   = errors.New("forbidden to edit this step")
	ErrSessionUndone   = errors.New("forbidden to read undone session")
	ErrInvalidInput    = errors.New("invalid input body or query params")
)
