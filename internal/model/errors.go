package model

import "errors"

var (
	ErrIdOverflow       = errors.New("no more available ids for new sessions")
	ErrNoSession        = errors.New("there is no session with required id")
	ErrValidationError  = errors.New("some of your data is invalid")
	ErrUserForbidden    = errors.New("there is no user with required id in this session")
	ErrStepForbidden    = errors.New("forbidden to edit this step")
	ErrSessionUndone    = errors.New("forbidden to read undone session")
	ErrInvalidInput     = errors.New("invalid input body or query params")
	ErrCollision        = errors.New("data with given key already exists")
	ErrNoUser           = errors.New("user with required name not exists")
	ErrUserUnauthorized = errors.New("user authorization error")
	ErrWrongPassword    = errors.New("wrong password of user with required name")
	ErrNoInviteCode     = errors.New("given invite code not exists")
)
