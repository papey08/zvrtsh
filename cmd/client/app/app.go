package app

import (
	"net/http"
)

type App struct {
	cli     *http.Client
	baseUrl string

	name       string
	secretCode string

	SessionCode string
	StepsAmount int
	CurrentStep int
}

func New(addr string) App {
	return App{
		cli:     &http.Client{},
		baseUrl: addr + "/zvrtsh/api/v1/",
	}
}
