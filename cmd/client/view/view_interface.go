package view

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
