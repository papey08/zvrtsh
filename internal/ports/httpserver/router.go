package httpserver

import (
	"zvrtsh/internal/app"

	"github.com/gin-gonic/gin"
)

func appRouter(r *gin.RouterGroup, a app.App) {
	r.POST("/session", createSession(a))
	r.GET("/session", getSession(a))
	r.POST("/question", makeQuestion(a))
	r.POST("/answer", makeAnswer(a))
	r.GET("/question", getQuestion(a))
	r.GET("/results", getResults(a))

	r.POST("/user", registerUser(a))
	r.GET("/invite/:name", getUserInviteCode(a))
	r.GET("/user/:name", getUserSecretCode(a))
}
