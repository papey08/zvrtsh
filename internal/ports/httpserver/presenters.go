package httpserver

type createSessionRequest struct {
	Id01        string `json:"user_01_id"`
	Id02        string `json:"user_02_id"`
	StepsAmount int    `json:"steps_amount"`
}

type makeQuestionRequest struct {
	SessionId      uint64 `json:"session_id"`
	StepIdx        int    `json:"step_index"`
	HiddenQuestion string `json:"hidden_question"`
	OpenQuestion   string `json:"open_question"`
}

type makeAnswerRequest struct {
	SessionId uint64 `json:"session_id"`
	StepIdx   int    `json:"step_index"`
	Answer    string `json:"answer"`
}

type registerUserRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type getUserSecretCodeRequest struct {
	Password string `json:"passsword"`
}
