package httpserver

type CreateSessionRequest struct {
	Id01        string `json:"user_01_id"`
	Id02        string `json:"user_02_id"`
	StepsAmount int    `json:"steps_amount"`
}

type MakeQuestionRequest struct {
	SessionId      uint64 `json:"session_id"`
	StepIdx        int    `json:"step_index"`
	HiddenQuestion string `json:"hidden_question"`
	OpenQuestion   string `json:"open_question"`
}

type MakeAnswerRequest struct {
	SessionId uint64 `json:"session_id"`
	StepIdx   int    `json:"step_index"`
	Answer    string `json:"answer"`
}

type RegisterUserRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type GetUserSecretCodeRequest struct {
	Password string `json:"passsword"`
}
