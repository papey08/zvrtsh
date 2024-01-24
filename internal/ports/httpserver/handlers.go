package httpserver

import (
	"errors"
	"net/http"
	"strconv"
	"zvrtsh/internal/app"
	"zvrtsh/internal/model"

	"github.com/gin-gonic/gin"
)

func createSession(a app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createSessionRequest
		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(model.ErrInvalidInput))
			return
		}

		sessionId, err := a.CreateSession(c, req.Id01, req.Id02, req.StepsAmount)
		switch {
		case errors.Is(err, model.ErrIdOverflow):
			c.AbortWithStatusJSON(http.StatusInsufficientStorage, errorResponse(err))
		case errors.Is(err, model.ErrValidationError):
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(err))
		case err == nil:
			c.JSON(http.StatusOK, successResponse(createSessionData{sessionId}))
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse(err))
		}
	}
}

func makeQuestion(a app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req makeQuestionRequest
		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(model.ErrInvalidInput))
			return
		}

		err := a.MakeQuestion(c, req.SessionId, req.UserId, req.StepIdx, req.HiddenQuestion, req.OpenQuestion)
		switch {
		case errors.Is(err, model.ErrValidationError):
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(err))
		case errors.Is(err, model.ErrNoSession):
			c.AbortWithStatusJSON(http.StatusNotFound, errorResponse(err))
		case errors.Is(err, model.ErrUserForbidden):
			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse(err))
		case errors.Is(err, model.ErrStepForbidden):
			c.AbortWithStatusJSON(http.StatusConflict, errorResponse(err))
		case err == nil:
			c.JSON(http.StatusOK, successResponse(nil))
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse(err))
		}
	}
}

func makeAnswer(a app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req makeAnswerRequest
		if err := c.BindJSON(&req); err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(model.ErrInvalidInput))
			return
		}

		err := a.MakeAnswer(c, req.SessionId, req.UserId, req.StepIdx, req.Answer)
		switch {
		case errors.Is(err, model.ErrValidationError):
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(err))
		case errors.Is(err, model.ErrNoSession):
			c.AbortWithStatusJSON(http.StatusNotFound, errorResponse(err))
		case errors.Is(err, model.ErrUserForbidden):
			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse(err))
		case errors.Is(err, model.ErrStepForbidden):
			c.AbortWithStatusJSON(http.StatusConflict, errorResponse(err))
		case err == nil:
			c.JSON(http.StatusOK, successResponse(nil))
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse(err))
		}
	}
}

func getQuestion(a app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionIdStr := c.Query("session_id")
		userIdStr := c.Query("user_id")
		stepIdxStr := c.Query("step_idx")

		sessionId, err := strconv.Atoi(sessionIdStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(model.ErrInvalidInput))
			return
		}
		userId, err := strconv.Atoi(userIdStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(model.ErrInvalidInput))
			return
		}
		stepIdx, err := strconv.Atoi(stepIdxStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(model.ErrInvalidInput))
			return
		}

		openQuestion, err := a.GetQuestion(c, uint(sessionId), uint(userId), stepIdx)
		switch {
		case errors.Is(err, model.ErrValidationError):
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(err))
		case errors.Is(err, model.ErrNoSession):
			c.AbortWithStatusJSON(http.StatusNotFound, errorResponse(err))
		case errors.Is(err, model.ErrUserForbidden):
			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse(err))
		case errors.Is(err, model.ErrStepForbidden):
			c.AbortWithStatusJSON(http.StatusConflict, errorResponse(err))
		case err == nil:
			c.JSON(http.StatusOK, successResponse(getQuestionData{openQuestion}))
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse(err))
		}
	}
}

func getResults(a app.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionIdStr := c.Query("session_id")
		userIdStr := c.Query("user_id")

		sessionId, err := strconv.Atoi(sessionIdStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(model.ErrInvalidInput))
			return
		}
		userId, err := strconv.Atoi(userIdStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, errorResponse(model.ErrInvalidInput))
			return
		}

		session, err := a.GetResults(c, uint(sessionId), uint(userId))
		switch {
		case errors.Is(err, model.ErrNoSession):
			c.AbortWithStatusJSON(http.StatusNotFound, errorResponse(err))
		case errors.Is(err, model.ErrUserForbidden):
			c.AbortWithStatusJSON(http.StatusForbidden, errorResponse(err))
		case errors.Is(err, model.ErrSessionUndone):
			c.AbortWithStatusJSON(http.StatusConflict, errorResponse(err))
		case err == nil:
			c.JSON(http.StatusOK, successSessionResponse(session))
		default:
			c.AbortWithStatusJSON(http.StatusInternalServerError, errorResponse(err))
		}
	}
}
