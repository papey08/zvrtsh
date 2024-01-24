package httpserver

import (
	"net/http"
	"zvrtsh/internal/app"

	"github.com/gin-gonic/gin"
)

func New(addr string, a app.App) *http.Server {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()
	api := router.Group("zvrtsh/api/v1")
	appRouter(api, a)
	return &http.Server{
		Addr:    addr,
		Handler: router,
	}
}
