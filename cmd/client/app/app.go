package app

import (
	"net/http"
	"time"
)

type App struct {
	cli     *http.Client
	baseUrl string

	name       string
	secretCode uint64

	SessionCode uint64
	StepsAmount int
	CurrentStep int
}

func New(addr string) App {
	return App{
		cli: &http.Client{
			Timeout: time.Second * 10,
		},
		baseUrl: addr + "/zvrtsh/api/v1/",
	}
}
