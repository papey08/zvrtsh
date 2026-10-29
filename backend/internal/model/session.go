package model

import "time"

type Session struct {
	Id         string
	Sheets     [2]Sheet // the sheet 0 belongs to the player who has created the session
	CreatedAt  time.Time
	LastActive time.Time
	Version    int64 // grows with every change

	// FinishVotes are the offers of the players (by sheet index) to finish
	// the session after the current step.
	FinishVotes [2]bool
}

// Clone returns a deep copy, safe to be used without holding the storage lock.
func (s Session) Clone() Session {
	for i := range s.Sheets {
		s.Sheets[i].Steps = append([]Step(nil), s.Sheets[i].Steps...)
	}
	return s
}

// PlayerIdx returns the index of the sheet owned by the given user.
func (s Session) PlayerIdx(userName string) (int, bool) {
	for i, sheet := range s.Sheets {
		if sheet.Owner == userName {
			return i, true
		}
	}
	return 0, false
}

// QuestionSheetIdx is the sheet the player writes the question of the step on:
// the own one on even steps and the opponent's one on odd steps.
func QuestionSheetIdx(stepIdx int, playerIdx int) int {
	if stepIdx%2 == 0 {
		return playerIdx
	}
	return 1 - playerIdx
}

// AnswerSheetIdx is the sheet the player answers on, always the opposite of QuestionSheetIdx.
func AnswerSheetIdx(stepIdx int, playerIdx int) int {
	return 1 - QuestionSheetIdx(stepIdx, playerIdx)
}

// StepProgress tells what is done on a step from the point of view of a player.
type StepProgress struct {
	MyQuestion       bool
	MyAnswer         bool
	OpponentQuestion bool
	OpponentAnswer   bool
}

// Progress returns the state of every step for the player with the given sheet index.
func (s Session) Progress(playerIdx int) []StepProgress {
	progress := make([]StepProgress, len(s.Sheets[0].Steps))
	for i := range progress {
		mine := s.Sheets[QuestionSheetIdx(i, playerIdx)].Steps[i].Status()
		opponents := s.Sheets[AnswerSheetIdx(i, playerIdx)].Steps[i].Status()
		progress[i] = StepProgress{
			MyQuestion:       mine != StepWaiting,
			MyAnswer:         opponents == StepAnswered,
			OpponentQuestion: opponents != StepWaiting,
			OpponentAnswer:   mine == StepAnswered,
		}
	}
	return progress
}

// CurrentStep is the step the players are busy with: the latest one anybody
// has written something on, or the first unfinished one if it is later.
// Finishing after it loses nothing written and makes nobody start a new question.
func (s Session) CurrentStep() int {
	current := 0
	for i := range s.Sheets[0].Steps {
		touched := s.Sheets[0].Steps[i].Status() != StepWaiting || s.Sheets[1].Steps[i].Status() != StepWaiting
		if touched {
			current = i
		}
	}
	for i := range s.Sheets[0].Steps {
		if s.Sheets[0].Steps[i].Status() != StepAnswered || s.Sheets[1].Steps[i].Status() != StepAnswered {
			return max(current, i)
		}
	}
	return current
}

// Shorten leaves only the first n steps on both sheets.
func (s *Session) Shorten(n int) {
	for i := range s.Sheets {
		s.Sheets[i].Steps = s.Sheets[i].Steps[:n]
	}
}

func (s Session) Status() SessionStatus {
	for _, sheet := range s.Sheets {
		for _, step := range sheet.Steps {
			if step.Status() != StepAnswered {
				return StatusInWork
			}
		}
	}
	return StatusDone
}

type SessionStatus int

const (
	StatusInWork SessionStatus = iota + 1
	StatusDone
)

type Sheet struct {
	Owner string
	Steps []Step
}

type Step struct {
	HiddenQuestion string
	OpenQuestion   string
	Answer         string
}

func (s Step) Status() StepStatus {
	switch {
	case s.OpenQuestion == "" && s.Answer == "":
		return StepWaiting
	case s.OpenQuestion != "" && s.Answer == "":
		return StepAnswering
	default:
		return StepAnswered
	}
}

type StepStatus int

const (
	StepWaiting StepStatus = iota + 1
	StepAnswering
	StepAnswered
)
