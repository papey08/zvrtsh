package httpserver

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"zvrtsh/internal/model"
	"zvrtsh/internal/ports/httpserver/api"

	"github.com/gin-gonic/gin"
)

// errBadRequest marks errors caused by a malformed request (broken JSON, wrong path parameter).
func errBadRequest(err error) error {
	return fmt.Errorf("%w: %s", model.ErrValidation, err.Error())
}

// writeError converts an error of the application into a response described in api/openapi.yaml.
// Details of unexpected errors are only logged and never sent to the client.
func writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	message := "sorry, something wrong with the server"

	switch {
	case errors.Is(err, model.ErrValidation):
		status, message = http.StatusBadRequest, err.Error()
	case errors.Is(err, model.ErrUnauthorized):
		status, message = http.StatusUnauthorized, model.ErrUnauthorized.Error()
		c.Header("WWW-Authenticate", "Bearer")
	case errors.Is(err, model.ErrNotParticipant):
		status, message = http.StatusForbidden, model.ErrNotParticipant.Error()
	case errors.Is(err, model.ErrNotHost):
		status, message = http.StatusForbidden, model.ErrNotHost.Error()
	case errors.Is(err, model.ErrNoSession):
		status, message = http.StatusNotFound, model.ErrNoSession.Error()
	case errors.Is(err, model.ErrNoInviteCode):
		status, message = http.StatusNotFound, model.ErrNoInviteCode.Error()
	case errors.Is(err, model.ErrUserExists):
		status, message = http.StatusConflict, model.ErrUserExists.Error()
	case errors.Is(err, model.ErrStepUnavailable):
		status, message = http.StatusConflict, model.ErrStepUnavailable.Error()
	case errors.Is(err, model.ErrSessionDone):
		status, message = http.StatusConflict, model.ErrSessionDone.Error()
	case errors.Is(err, model.ErrSessionUndone):
		status, message = http.StatusConflict, model.ErrSessionUndone.Error()
	case errors.Is(err, model.ErrSessionStarted):
		status, message = http.StatusConflict, model.ErrSessionStarted.Error()
	case errors.Is(err, model.ErrSessionNotStarted):
		status, message = http.StatusConflict, model.ErrSessionNotStarted.Error()
	case errors.Is(err, model.ErrSessionFull):
		status, message = http.StatusConflict, model.ErrSessionFull.Error()
	case errors.Is(err, model.ErrTooFewPlayers):
		status, message = http.StatusConflict, model.ErrTooFewPlayers.Error()
	default:
		log.Printf("internal error: %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	}

	c.AbortWithStatusJSON(status, api.Error{Error: message})
}
