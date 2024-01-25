package httpserver

import "zvrtsh/internal/model"

type createSessionData struct {
	SessionId uint64 `json:"session_id"`
}

type getQuestionData struct {
	OpenQuestion string `json:"open_question"`
}

type response struct {
	Data any     `json:"data"`
	Err  *string `json:"error"`
}

func successResponse(data any) response {
	return response{
		Data: data,
		Err:  nil,
	}
}

func errorResponse(err error) response {
	errStr := err.Error()
	return response{
		Data: nil,
		Err:  &errStr,
	}
}

type getSessionData struct {
	Id     uint64   `json:"id"`
	Sheets [2]sheet `json:"sheets"`
}

type sheet struct {
	OwnerId uint64 `json:"owner_id"`
	Steps   []step `json:"steps"`
}

type step struct {
	HiddenQuestion string `json:"hidden_question"`
	OpenQuestion   string `json:"open_question"`
	Answer         string `json:"answer"`
}

func successSessionResponse(s model.Session) response {
	var data getSessionData
	data.Id = s.Id
	for i := 0; i < 2; i++ {
		data.Sheets[i].OwnerId = s.Sheets[i].OwnerId
		data.Sheets[i].Steps = make([]step, len(s.Sheets[i].Steps))
		for j, step := range s.Sheets[i].Steps {
			data.Sheets[i].Steps[j].HiddenQuestion = step.HiddenQuestion
			data.Sheets[i].Steps[j].OpenQuestion = step.OpenQuestion
			data.Sheets[i].Steps[j].Answer = step.Answer
		}
	}
	return successResponse(data)
}

type userData struct {
	Name       string `json:"name"`
	SecretCode uint64 `json:"access_token"`
}

type userInviteCodeData struct {
	InviteCode string `json:"invite_code"`
}

type userSecretCodeData struct {
	SecretCode uint64 `json:"secret_code"`
}
