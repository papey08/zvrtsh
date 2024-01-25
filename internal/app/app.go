package app

import inviterepo "zvrtsh/internal/repo/invite_repo"

func New() App {
	invites := inviterepo.New()
	return &app{
		sessionApp: newSessionApp(invites),
		userApp:    newUserApp(invites),
	}
}

type app struct {
	sessionApp
	userApp
}
