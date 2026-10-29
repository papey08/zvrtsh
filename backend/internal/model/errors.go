package model

import "errors"

var (
	// ErrValidation means that the given data is invalid (HTTP 400).
	ErrValidation = errors.New("some of your data is invalid")

	// ErrUnauthorized means that credentials or a token are missing or wrong (HTTP 401).
	ErrUnauthorized = errors.New("invalid credentials or token")

	// ErrNotParticipant means that the user is not a player of the session (HTTP 403).
	ErrNotParticipant = errors.New("you are not a player of this session")

	// ErrNoSession and ErrNoInviteCode mean that the required entity does not exist (HTTP 404).
	ErrNoSession    = errors.New("there is no session with required id")
	ErrNoInviteCode = errors.New("invite code does not exist or has expired")

	// ErrUserNotFound is returned by repositories only and never reaches the client as is.
	ErrUserNotFound = errors.New("user not found")

	// ErrUserExists, ErrStepUnavailable and ErrSessionUndone mean that the request
	// conflicts with the current state of the entity (HTTP 409).
	ErrUserExists      = errors.New("user with given name already exists")
	ErrStepUnavailable = errors.New("step is not available right now")
	ErrSessionUndone   = errors.New("session is not finished yet")
	ErrSessionDone     = errors.New("session is already finished")

	// ErrDatabase wraps unexpected storage errors (HTTP 500).
	ErrDatabase = errors.New("database error")
)
