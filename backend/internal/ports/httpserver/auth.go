package httpserver

import (
	"context"
	"net/http"
	"strings"
	"zvrtsh/internal/app"
	"zvrtsh/internal/model"
	"zvrtsh/internal/ports/httpserver/api"

	"github.com/gin-gonic/gin"
)

const userNameKey = "user_name"

// publicRoutes are the only routes available without a token, all the
// others require it. It must match the operations without `security` in api/openapi.yaml.
var publicRoutes = map[string]bool{
	http.MethodPost + " " + basePath + "/users":      true,
	http.MethodPost + " " + basePath + "/auth/token": true,
	// the token is sent inside the WebSocket, see eventsHandler
	http.MethodGet + " " + eventsPath: true,
}

func authMiddleware(a app.App) api.MiddlewareFunc {
	return func(c *gin.Context) {
		if publicRoutes[c.Request.Method+" "+c.FullPath()] {
			return
		}

		scheme, token, _ := strings.Cut(c.GetHeader("Authorization"), " ")
		if !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
			writeError(c, model.ErrUnauthorized)
			return
		}

		userName, err := a.Authenticate(c, strings.TrimSpace(token))
		if err != nil {
			writeError(c, err)
			return
		}
		c.Set(userNameKey, userName)
	}
}

// userName returns the name of the authenticated user. The context passed to
// strict handlers is the gin context, which exposes its keys through Value.
func userName(ctx context.Context) string {
	name, _ := ctx.Value(userNameKey).(string)
	return name
}
