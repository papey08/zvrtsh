package httpserver

import (
	"context"
	"errors"
	"log"
	"time"
	"zvrtsh/internal/app"
	"zvrtsh/internal/model"
	"zvrtsh/internal/ports/httpserver/api"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
)

// eventsPath is the WebSocket endpoint described as subscribeEvents in api/openapi.yaml.
const eventsPath = basePath + "/events"

const (
	maxAuthMessageBytes = 4 << 10
	writeTimeout        = 10 * time.Second
)

type eventsConfig struct {
	authTimeout  time.Duration
	pingInterval time.Duration
}

var defaultEventsConfig = eventsConfig{
	authTimeout:  5 * time.Second,
	pingInterval: 25 * time.Second,
}

// eventsHandler streams updated sessions of the user over a WebSocket until the
// client leaves or serverCtx is done.
func eventsHandler(serverCtx context.Context, a app.App, cfg eventsConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Accept checks that Origin matches Host and answers by itself on failure
		conn, err := websocket.Accept(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(maxAuthMessageBytes)
		defer closeWhenDone(serverCtx, conn, websocket.StatusGoingAway, "server is shutting down")()

		userName, err := authenticateSocket(c.Request.Context(), conn, a, cfg.authTimeout)
		if err != nil {
			closeOnAuthError(conn, err)
			return
		}

		// subscribe before ready: everything after it reaches the client
		sessions, unsubscribe := a.SubscribeSessions(c.Request.Context(), userName)
		defer unsubscribe()

		// the client sends nothing more; reading in background handles pings and
		// close frames, ctx is done when the connection is closed by any side
		ctx := conn.CloseRead(c.Request.Context())
		if err = writeMessage(ctx, conn, api.EventsServerMessage{Type: api.Ready}); err != nil {
			return
		}

		ticker := time.NewTicker(cfg.pingInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case session, ok := <-sessions:
				if !ok {
					_ = conn.Close(websocket.StatusTryAgainLater, "too many events, reconnect")
					return
				}
				view := toSession(session, userName)
				if err = writeMessage(ctx, conn, api.EventsServerMessage{Type: api.SessionUpdated, Session: &view}); err != nil {
					return
				}
			case <-ticker.C:
				pingCtx, cancelPing := context.WithTimeout(ctx, writeTimeout)
				err = conn.Ping(pingCtx)
				cancelPing()
				if err != nil {
					return
				}
			}
		}
	}
}

// authenticateSocket waits for the auth message and returns the name of the user.
func authenticateSocket(ctx context.Context, conn *websocket.Conn, a app.App, timeout time.Duration) (string, error) {
	// not a read deadline: a cancelled read drops the connection without a close frame
	timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	stop := closeWhenDone(timeoutCtx, conn, websocket.StatusPolicyViolation, "authentication timeout")

	var msg api.EventsAuthMessage
	err := wsjson.Read(ctx, conn, &msg)
	stop()
	if err != nil {
		return "", errors.Join(model.ErrUnauthorized, err)
	}
	if msg.Type != api.Auth {
		return "", model.ErrUnauthorized
	}
	return a.Authenticate(ctx, msg.Token)
}

// closeWhenDone closes the connection with the status as soon as ctx is done.
// The returned function cancels that or waits until the started closing ends.
func closeWhenDone(ctx context.Context, conn *websocket.Conn, code websocket.StatusCode, reason string) func() {
	closed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		defer close(closed)
		_ = conn.Close(code, reason)
	})
	return func() {
		if !stop() {
			<-closed
		}
	}
}

func closeOnAuthError(conn *websocket.Conn, err error) {
	if errors.Is(err, model.ErrUnauthorized) {
		_ = conn.Close(websocket.StatusPolicyViolation, model.ErrUnauthorized.Error())
		return
	}
	log.Printf("internal error: websocket authentication: %v", err)
	_ = conn.Close(websocket.StatusInternalError, "sorry, something wrong with the server")
}

func writeMessage(ctx context.Context, conn *websocket.Conn, msg api.EventsServerMessage) error {
	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	return wsjson.Write(ctx, conn, msg)
}
