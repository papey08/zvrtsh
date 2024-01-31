package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"zvrtsh/internal/app"
	"zvrtsh/internal/ports/httpserver"
	inviterepo "zvrtsh/internal/repo/invite_repo"
	userrepo "zvrtsh/internal/repo/user_repo"
)

const addr = "zvrtsh-app:8080"

func main() {
	inviteRepo := inviterepo.New()
	userRepo, err := userrepo.New(context.Background(),
		fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			"root",
			"root",
			"postgres-users",
			5432,
			"postgres",
			"disable",
		),
	)
	defer userRepo.Close()
	if err != nil {
		log.Fatal(err.Error())
	}

	a := app.New(inviteRepo, userRepo)
	srv := httpserver.New(addr, a)

	// preparing graceful shutdown
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGINT)

	go func() {
		log.Println("Starting http server")
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("can't listen and serve server:", err.Error())
		}
	}()

	// waiting for Ctrl+C
	<-osSignals

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30s timeout to finish all active connections
	defer cancel()

	if err = srv.Shutdown(ctx); err != nil {
		log.Fatal("Server graceful shutdown failed:", err.Error())
	}
	log.Println("Server was gracefully stopped")
}
