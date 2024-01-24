package httpserver

type createSessionRequest struct {
	Id01        uint `json:"user_01_id"`
	Id02        uint `json:"user_02_id"`
	StepsAmount int  `json:"steps_amount"`
}

type makeQuestionRequest struct {
	SessionId      uint   `json:"session_id"`
	UserId         uint   `json:"user_id"`
	StepIdx        int    `json:"step_index"`
	HiddenQuestion string `json:"hidden_question"`
	OpenQuestion   string `json:"open_question"`
}

type makeAnswerRequest struct {
	SessionId uint   `json:"session_id"`
	UserId    uint   `json:"user_id"`
	StepIdx   int    `json:"step_index"`
	Answer    string `json:"answer"`
}
