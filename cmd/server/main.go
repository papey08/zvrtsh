package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
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
	viper.SetConfigFile("config/config.yml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("cannot read config file: ", err.Error())
	}
	if err := godotenv.Load("config/.env"); err != nil {
		log.Fatal("unable to load .env file: ", err.Error())
	}

	inviteRepo := inviterepo.New()
	userRepo, err := userrepo.New(context.Background(),
		fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			viper.GetString("users-db.user"),
			os.Getenv("USERS_DB_PASSWORD"),
			viper.GetString("users-db.host"),
			viper.GetInt("users-db.port"),
			viper.GetString("users-db.db"),
			viper.GetString("users-db.sslmode"),
		),
	)
	if err != nil {
		log.Fatal("cannot connect to the database: ", err.Error())
	}
	defer userRepo.Close()

	a := app.New(inviteRepo, userRepo, os.Getenv("USER_PASSWORD_SALT"))
	srv := httpserver.New(addr, a)

	// preparing graceful shutdown
	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGTERM)
	signal.Notify(osSignals, os.Interrupt, syscall.SIGINT)

	go func() {
		log.Println("Starting http server")
		if err = srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("can't listen and serve server:", err.Error())
		}
	}()

	// waiting for Ctrl+C
	<-osSignals

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30s timeout to finish all active connections
	defer cancel()

	if err = srv.Shutdown(ctx); err != nil {
		log.Fatal("Server graceful shutdown failed: ", err.Error())
	}
	log.Println("Server was gracefully stopped")
}
