package model

type Session struct {
	Id     string
	Sheets [2]Sheet
}

func (s Session) Status() SessionStatus {
	lastStepIdx := len(s.Sheets[0].Steps) - 1
	if lastStepIdx == -1 {
		return StatusDone
	} else if s.Sheets[0].Steps[lastStepIdx].Status() == StatusAnswered && s.Sheets[1].Steps[lastStepIdx].Status() == StatusAnswered {
		return StatusDone
	}
	return StatusInWork
}

type SessionStatus int

const (
	StatusInWork SessionStatus = iota + 1
	StatusDone
)

type Sheet struct {
	OwnerId string
	Steps   []Step
}

type Step struct {
	HiddenQuestion string
	OpenQuestion   string
	Answer         string
}

func (s Step) Status() StepStatus {
	if s.HiddenQuestion == "" && s.OpenQuestion == "" && s.Answer == "" {
		return StatusWaiting
	} else if s.HiddenQuestion != "" && s.OpenQuestion != "" && s.Answer == "" {
		return StatusAnswering
	} else if s.HiddenQuestion != "" && s.OpenQuestion != "" && s.Answer != "" {
		return StatusAnswered
	} else {
		return StatusUnknown
	}
}

type StepStatus int

const (
	StatusWaiting StepStatus = iota + 1
	StatusAnswering
	StatusAnswered
	StatusUnknown
)
