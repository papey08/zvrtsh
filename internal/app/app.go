package app

import (
	inviterepo "zvrtsh/internal/repo/invite_repo"
	userrepo "zvrtsh/internal/repo/user_repo"
)

func New(invites inviterepo.InviteRepo, users userrepo.UserRepo) App {
	return &app{
		sessionApp: newSessionApp(invites),
		userApp:    newUserApp(users, invites),
	}
}

type app struct {
	sessionApp
	userApp
}
