package view

import defaultview "zvrtsh/cmd/client/view/default_view"

type View interface {
	Start()
	SetHost()
	Registration()
	Login()
	ShowInviteCode()
	EnterSession()
	CreateSession()
	GetQuestion()
	ShowQuestion()
	GetAnswer()
	ShowResults()
}

func New() View {
	return &defaultview.ViewImpl{}
}
