package main

import (
	"context"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"zvrtsh/internal/app"
	"zvrtsh/internal/ports/httpserver"
	"zvrtsh/internal/repo/postgres"
	sessionrepo "zvrtsh/internal/repo/session_repo"
	userrepo "zvrtsh/internal/repo/user_repo"

	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigFile("config/config.yml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("cannot read config file: ", err.Error())
	}
	// any value of the config can be overridden by an environment variable,
	// e.g. users-db.host by USERS_DB_HOST
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, usersDBAddr())
	if err != nil {
		log.Fatal("cannot connect to the database: ", err.Error())
	}
	defer pool.Close()

	a := app.New(ctx, app.Config{
		InviteTTL:  viper.GetDuration("invite-ttl"),
		SessionTTL: viper.GetDuration("session-ttl"),
	}, userrepo.New(pool), sessionrepo.New(pool))

	addr := net.JoinHostPort(viper.GetString("http-server.host"), strconv.Itoa(viper.GetInt("http-server.port")))
	srv := httpserver.New(ctx, addr, a)

	serveErr := make(chan error, 1)
	go func() {
		log.Println("Starting http server on", addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err = <-serveErr:
		log.Fatal("can't listen and serve server: ", err.Error())
	case <-ctx.Done(): // Ctrl+C or SIGTERM
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30s timeout to finish all active connections
	defer cancel()

	if err = srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server graceful shutdown failed: ", err.Error())
	}
	log.Println("Server was gracefully stopped")
}

func usersDBAddr() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(viper.GetString("users-db.user"), viper.GetString("users-db.password")),
		Host:   net.JoinHostPort(viper.GetString("users-db.host"), strconv.Itoa(viper.GetInt("users-db.port"))),
		Path:   viper.GetString("users-db.db"),
	}
	q := url.Values{}
	q.Set("sslmode", viper.GetString("users-db.sslmode"))
	u.RawQuery = q.Encode()
	return u.String()
}
