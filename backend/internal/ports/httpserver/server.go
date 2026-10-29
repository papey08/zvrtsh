package httpserver

import (
	"context"
	"net/http"
	"time"
	"zvrtsh/internal/app"
	"zvrtsh/internal/ports/httpserver/api"

	"github.com/gin-gonic/gin"
)

// basePath must match the server url in api/openapi.yaml.
const basePath = "/zvrtsh/api/v1"

const maxBodyBytes = 64 << 10

// New creates the server. WebSocket connections are closed when ctx is done:
// Shutdown does not wait for them.
func New(ctx context.Context, addr string, a app.App) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           newRouter(ctx, a),
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func newRouter(ctx context.Context, a app.App) *gin.Engine {
	return newRouterWithEvents(ctx, a, defaultEventsConfig)
}

func newRouterWithEvents(ctx context.Context, a app.App, events eventsConfig) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), limitBody)

	strictHandler := api.NewStrictHandlerWithOptions(newHandlers(a), nil, api.StrictGinServerOptions{
		RequestErrorHandlerFunc:  func(c *gin.Context, err error) { writeError(c, errBadRequest(err)) },
		HandlerErrorFunc:         writeError,
		ResponseErrorHandlerFunc: writeError,
	})

	api.RegisterHandlersWithOptions(router, strictHandler, api.GinServerOptions{
		BaseURL:      basePath,
		Middlewares:  []api.MiddlewareFunc{authMiddleware(a)},
		ErrorHandler: func(c *gin.Context, err error, _ int) { writeError(c, errBadRequest(err)) },
	})
	router.GET(eventsPath, eventsHandler(ctx, a, events))

	router.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, api.Error{Error: "not found"})
	})
	router.NoMethod(func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, api.Error{Error: "method not allowed"})
	})
	router.HandleMethodNotAllowed = true
	return router
}

func limitBody(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)
	c.Next()
}
