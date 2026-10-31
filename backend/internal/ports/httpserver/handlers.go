package httpserver

import (
	"context"
	"zvrtsh/internal/app"
	"zvrtsh/internal/model"
	"zvrtsh/internal/ports/httpserver/api"
)

// handlers implements api.StrictServerInterface. Successful responses are typed
// by the generated code, errors are returned as is and converted by writeError.
type handlers struct {
	app app.App
}

var _ api.StrictServerInterface = (*handlers)(nil)

func newHandlers(a app.App) *handlers {
	return &handlers{app: a}
}

func (h *handlers) RegisterUser(ctx context.Context, request api.RegisterUserRequestObject) (api.RegisterUserResponseObject, error) {
	user, err := h.app.RegisterUser(ctx, request.Body.Name, request.Body.Password)
	if err != nil {
		return nil, err
	}
	return api.RegisterUser201JSONResponse{Name: user.Name}, nil
}

func (h *handlers) CreateToken(ctx context.Context, request api.CreateTokenRequestObject) (api.CreateTokenResponseObject, error) {
	token, err := h.app.CreateToken(ctx, request.Body.Name, request.Body.Password)
	if err != nil {
		return nil, err
	}
	return api.CreateToken200JSONResponse{Token: token}, nil
}

func (h *handlers) CreateInvite(ctx context.Context, request api.CreateInviteRequestObject) (api.CreateInviteResponseObject, error) {
	session, err := h.app.CreateInvite(ctx, userName(ctx), request.Body.StepsAmount)
	if err != nil {
		return nil, err
	}
	return api.CreateInvite201JSONResponse(toInvite(session)), nil
}

func (h *handlers) DeleteInvite(ctx context.Context, _ api.DeleteInviteRequestObject) (api.DeleteInviteResponseObject, error) {
	if err := h.app.DeleteInvite(ctx, userName(ctx)); err != nil {
		return nil, err
	}
	return api.DeleteInvite204Response{}, nil
}

func (h *handlers) CreateSession(ctx context.Context, request api.CreateSessionRequestObject) (api.CreateSessionResponseObject, error) {
	session, err := h.app.JoinSession(ctx, userName(ctx), request.Body.InviteCode)
	if err != nil {
		return nil, err
	}
	return api.CreateSession201JSONResponse(toSession(session, userName(ctx))), nil
}

func (h *handlers) StartSession(ctx context.Context, request api.StartSessionRequestObject) (api.StartSessionResponseObject, error) {
	session, err := h.app.StartSession(ctx, request.SessionId, userName(ctx))
	if err != nil {
		return nil, err
	}
	return api.StartSession200JSONResponse(toSession(session, userName(ctx))), nil
}

func (h *handlers) ListSessions(ctx context.Context, _ api.ListSessionsRequestObject) (api.ListSessionsResponseObject, error) {
	sessions, err := h.app.ListSessions(ctx, userName(ctx))
	if err != nil {
		return nil, err
	}
	response := make(api.ListSessions200JSONResponse, len(sessions))
	for i, session := range sessions {
		response[i] = toSession(session, userName(ctx))
	}
	return response, nil
}

func (h *handlers) GetSession(ctx context.Context, request api.GetSessionRequestObject) (api.GetSessionResponseObject, error) {
	session, err := h.app.GetSession(ctx, request.SessionId, userName(ctx))
	if err != nil {
		return nil, err
	}
	return api.GetSession200JSONResponse(toSession(session, userName(ctx))), nil
}

func (h *handlers) MakeQuestion(ctx context.Context, request api.MakeQuestionRequestObject) (api.MakeQuestionResponseObject, error) {
	err := h.app.MakeQuestion(ctx, request.SessionId, userName(ctx), request.StepIndex,
		request.Body.HiddenQuestion, request.Body.OpenQuestion)
	if err != nil {
		return nil, err
	}
	return api.MakeQuestion204Response{}, nil
}

func (h *handlers) GetQuestionToAnswer(ctx context.Context, request api.GetQuestionToAnswerRequestObject) (api.GetQuestionToAnswerResponseObject, error) {
	openQuestion, err := h.app.GetQuestionToAnswer(ctx, request.SessionId, userName(ctx), request.StepIndex)
	if err != nil {
		return nil, err
	}
	return api.GetQuestionToAnswer200JSONResponse{OpenQuestion: openQuestion}, nil
}

func (h *handlers) MakeAnswer(ctx context.Context, request api.MakeAnswerRequestObject) (api.MakeAnswerResponseObject, error) {
	err := h.app.MakeAnswer(ctx, request.SessionId, userName(ctx), request.StepIndex, request.Body.Answer)
	if err != nil {
		return nil, err
	}
	return api.MakeAnswer204Response{}, nil
}

func (h *handlers) VoteFinish(ctx context.Context, request api.VoteFinishRequestObject) (api.VoteFinishResponseObject, error) {
	session, err := h.app.VoteFinish(ctx, request.SessionId, userName(ctx), true)
	if err != nil {
		return nil, err
	}
	return api.VoteFinish200JSONResponse(toSession(session, userName(ctx))), nil
}

func (h *handlers) CancelFinishVote(ctx context.Context, request api.CancelFinishVoteRequestObject) (api.CancelFinishVoteResponseObject, error) {
	session, err := h.app.VoteFinish(ctx, request.SessionId, userName(ctx), false)
	if err != nil {
		return nil, err
	}
	return api.CancelFinishVote200JSONResponse(toSession(session, userName(ctx))), nil
}

func (h *handlers) GetResults(ctx context.Context, request api.GetResultsRequestObject) (api.GetResultsResponseObject, error) {
	session, err := h.app.GetResults(ctx, request.SessionId, userName(ctx))
	if err != nil {
		return nil, err
	}
	return api.GetResults200JSONResponse(toResults(session)), nil
}

// toSession converts the session as it is seen by the given player.
func toSession(s model.Session, userName string) api.Session {
	status := api.InWork
	switch s.Status() {
	case model.StatusLobby:
		status = api.Lobby
	case model.StatusDone:
		status = api.Done
	}

	names := s.PlayerNames()
	me, _ := s.PlayerIdx(userName)
	progress := make([]api.StepProgress, 0, s.StepsAmount)
	for _, p := range s.Progress(me) {
		progress = append(progress, api.StepProgress{
			MyQuestion:       p.MyQuestion,
			MyAnswer:         p.MyAnswer,
			AnswerSheetReady: p.AnswerSheetReady,
			AnswerSheetFrom:  names[p.AnswerSheetFrom],
			QuestionsWritten: p.QuestionsWritten,
			AnswersWritten:   p.AnswersWritten,
		})
	}

	votes := make([]string, 0, len(s.Players))
	for _, p := range s.Players {
		if p.FinishVote {
			votes = append(votes, p.Name)
		}
	}

	session := api.Session{
		Id:          s.Id,
		Version:     s.Version,
		Host:        s.Host(),
		Players:     names,
		StepsAmount: s.StepsAmount,
		Status:      status,
		Progress:    progress,
		FinishVotes: votes,
		UpdatedAt:   s.LastActive,
	}
	if s.Invite != nil {
		invite := toInvite(s)
		session.Invite = &invite
	}
	return session
}

// toInvite converts the invite of a session in the lobby.
func toInvite(s model.Session) api.Invite {
	return api.Invite{
		InviteCode:  s.Invite.Code,
		SessionId:   s.Id,
		StepsAmount: s.StepsAmount,
		ExpiresAt:   s.Invite.ExpiresAt,
	}
}

func toResults(s model.Session) api.Results {
	names := s.PlayerNames()
	results := api.Results{
		Id:     s.Id,
		Sheets: make([]api.Sheet, 0, len(s.Sheets)),
	}
	for i, sheet := range s.Sheets {
		steps := make([]api.Step, len(sheet.Steps))
		for j, step := range sheet.Steps {
			steps[j] = api.Step{
				HiddenQuestion: step.HiddenQuestion,
				OpenQuestion:   step.OpenQuestion,
				Answer:         step.Answer,
				Asker:          names[step.Asker],
				Answerer:       names[step.Answerer],
			}
		}
		results.Sheets = append(results.Sheets, api.Sheet{Owner: names[i], Steps: steps})
	}
	return results
}
