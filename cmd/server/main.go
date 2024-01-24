package main

import (
	"zvrtsh/internal/app"
	"zvrtsh/internal/ports/httpserver"
)

const addr = "localhost:8080"

func main() {
	a := app.New()
	srv := httpserver.New(addr, a)
	_ = srv.ListenAndServe()
}
