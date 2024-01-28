package httpserver

import "zvrtsh/internal/model"

type CreateSessionData struct {
	SessionId uint64 `json:"session_id"`
}

type GetQuestionData struct {
	OpenQuestion string `json:"open_question"`
}

type Response struct {
	Data any     `json:"data"`
	Err  *string `json:"error"`
}

func successResponse(data any) Response {
	return Response{
		Data: data,
		Err:  nil,
	}
}

func errorResponse(err error) Response {
	errStr := err.Error()
	return Response{
		Data: nil,
		Err:  &errStr,
	}
}

type GetSessionData struct {
	Id     uint64   `json:"id"`
	Sheets [2]Sheet `json:"sheets"`
}

type Sheet struct {
	OwnerId uint64 `json:"owner_id"`
	Steps   []Step `json:"steps"`
}

type Step struct {
	HiddenQuestion string `json:"hidden_question"`
	OpenQuestion   string `json:"open_question"`
	Answer         string `json:"answer"`
}

func successSessionResponse(s model.Session) Response {
	var data GetSessionData
	data.Id = s.Id
	for i := 0; i < 2; i++ {
		data.Sheets[i].OwnerId = s.Sheets[i].OwnerId
		data.Sheets[i].Steps = make([]Step, len(s.Sheets[i].Steps))
		for j, step := range s.Sheets[i].Steps {
			data.Sheets[i].Steps[j].HiddenQuestion = step.HiddenQuestion
			data.Sheets[i].Steps[j].OpenQuestion = step.OpenQuestion
			data.Sheets[i].Steps[j].Answer = step.Answer
		}
	}
	return successResponse(data)
}

type UserData struct {
	Name       string `json:"name"`
	SecretCode uint64 `json:"access_token"`
}

type UserInviteCodeData struct {
	InviteCode string `json:"invite_code"`
}

type UserSecretCodeData struct {
	SecretCode uint64 `json:"secret_code"`
}
