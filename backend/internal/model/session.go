package model

import (
	"math/rand/v2"
	"time"
)

const (
	MinPlayers = 2
	MaxPlayers = 8
)

type Session struct {
	Id          string
	Players     []Player // in the order of joining, Players[0] is the host
	StepsAmount int
	Started     bool
	Invite      *Invite // only in the lobby

	// Sheets appear when the session starts: the sheet i is started by the
	// player i with the question of the first step.
	Sheets []Sheet

	CreatedAt  time.Time
	LastActive time.Time
	Version    int64 // grows with every change
}

type Player struct {
	Name string

	// FinishVote is the offer to finish the session after the current step.
	FinishVote bool
}

// Clone returns a deep copy, safe to be used without holding the storage lock.
func (s Session) Clone() Session {
	s.Players = append([]Player(nil), s.Players...)
	if s.Invite != nil {
		invite := *s.Invite
		s.Invite = &invite
	}
	if s.Sheets != nil {
		sheets := make([]Sheet, len(s.Sheets))
		for i, sheet := range s.Sheets {
			sheets[i] = Sheet{Steps: append([]Step(nil), sheet.Steps...)}
		}
		s.Sheets = sheets
	}
	return s
}

func (s Session) Host() string {
	return s.Players[0].Name
}

func (s Session) PlayerNames() []string {
	names := make([]string, len(s.Players))
	for i, p := range s.Players {
		names[i] = p.Name
	}
	return names
}

// PlayerIdx returns the index of the player with the given name.
func (s Session) PlayerIdx(userName string) (int, bool) {
	for i, p := range s.Players {
		if p.Name == userName {
			return i, true
		}
	}
	return 0, false
}

// Start deals the sheets to the players for every step and starts the session.
func (s *Session) Start(r *rand.Rand) {
	s.Sheets = Deal(len(s.Players), s.StepsAmount, r)
	s.Started = true
	s.Invite = nil
}

// Deal creates empty sheets of the players with a random deal. Sheet i starts
// with the question of player i. When everybody has written the question, the
// sheets are shuffled by a random derangement, so nobody answers their own
// question. After the answer the sheet stays with the player, who writes the
// next question on it. With two players the sheets are simply swapped.
func Deal(players int, steps int, r *rand.Rand) []Sheet {
	sheets := make([]Sheet, players)
	holders := make([]int, players) // holders[sheet] is the player who writes on it now
	for i := range sheets {
		sheets[i].Steps = make([]Step, steps)
		holders[i] = i
	}

	for step := range steps {
		for sheet := range sheets {
			sheets[sheet].Steps[step].Asker = holders[sheet]
		}
		d := derangement(players, r)
		for sheet, player := range holders {
			holders[sheet] = d[player]
		}
		for sheet := range sheets {
			sheets[sheet].Steps[step].Answerer = holders[sheet]
		}
	}
	return sheets
}

// derangement returns a uniformly random permutation without fixed points:
// player i passes the sheet to player d[i] != i.
func derangement(n int, r *rand.Rand) []int {
	for {
		d := r.Perm(n)
		ok := true
		for i, j := range d {
			if i == j {
				ok = false
				break
			}
		}
		if ok { // happens with the probability of about 1/e
			return d
		}
	}
}

// QuestionSheetIdx is the sheet the player writes the question of the step on.
func (s Session) QuestionSheetIdx(stepIdx int, playerIdx int) int {
	for i, sheet := range s.Sheets {
		if sheet.Steps[stepIdx].Asker == playerIdx {
			return i
		}
	}
	return -1
}

// AnswerSheetIdx is the sheet the player answers the question of the step on.
func (s Session) AnswerSheetIdx(stepIdx int, playerIdx int) int {
	for i, sheet := range s.Sheets {
		if sheet.Steps[stepIdx].Answerer == playerIdx {
			return i
		}
	}
	return -1
}

// AnswersOpen tells whether the questions of the step can be answered: the
// sheets are shuffled only when everybody has written their question.
func (s Session) AnswersOpen(stepIdx int) bool {
	for _, sheet := range s.Sheets {
		if sheet.Steps[stepIdx].Status() == StepWaiting {
			return false
		}
	}
	return true
}

// StepProgress tells what is done on a step from the point of view of a player.
type StepProgress struct {
	// The question of a step is written on the sheet the player has answered
	// the previous step on, so it is available after MyAnswer of that step.
	MyQuestion bool
	MyAnswer   bool

	// AnswerSheetReady means that the question the player answers is dealt to
	// them: everybody has written their question. AnswerSheetFrom has written it.
	AnswerSheetReady bool
	AnswerSheetFrom  int

	QuestionsWritten int
	AnswersWritten   int
}

// Progress returns the state of every step for the player with the given index.
func (s Session) Progress(playerIdx int) []StepProgress {
	if !s.Started {
		return []StepProgress{}
	}
	progress := make([]StepProgress, s.StepsAmount)
	for i := range progress {
		qs := s.Sheets[s.QuestionSheetIdx(i, playerIdx)].Steps
		as := s.Sheets[s.AnswerSheetIdx(i, playerIdx)].Steps[i]

		p := StepProgress{
			MyQuestion:       qs[i].Status() != StepWaiting,
			MyAnswer:         as.Status() == StepAnswered,
			AnswerSheetReady: s.AnswersOpen(i),
			AnswerSheetFrom:  as.Asker,
		}
		for _, sheet := range s.Sheets {
			switch sheet.Steps[i].Status() {
			case StepAnswering:
				p.QuestionsWritten++
			case StepAnswered:
				p.QuestionsWritten++
				p.AnswersWritten++
			}
		}
		progress[i] = p
	}
	return progress
}

// CurrentStep is the step the players are busy with: the latest one anybody
// has written something on, or the first unfinished one if it is later.
// Finishing after it loses nothing written and makes nobody start a new question.
func (s Session) CurrentStep() int {
	current := 0
	for _, sheet := range s.Sheets {
		for i, step := range sheet.Steps {
			if step.Status() != StepWaiting {
				current = max(current, i)
			}
		}
	}
	for i := 0; i < s.StepsAmount; i++ {
		for _, sheet := range s.Sheets {
			if sheet.Steps[i].Status() != StepAnswered {
				return max(current, i)
			}
		}
	}
	return current
}

// Shorten leaves only the first n steps on all sheets.
func (s *Session) Shorten(n int) {
	s.StepsAmount = n
	for i := range s.Sheets {
		s.Sheets[i].Steps = s.Sheets[i].Steps[:n]
	}
}

// AllVotedToFinish tells whether every player offers to finish the session.
func (s Session) AllVotedToFinish() bool {
	for _, p := range s.Players {
		if !p.FinishVote {
			return false
		}
	}
	return true
}

func (s Session) Status() SessionStatus {
	if !s.Started {
		return StatusLobby
	}
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
	StatusLobby SessionStatus = iota + 1
	StatusInWork
	StatusDone
)

// SessionEvent is a change of a session delivered to its players.
type SessionEvent struct {
	Session Session
	Deleted bool // the host has cancelled the session in the lobby
}

type Sheet struct {
	Steps []Step
}

type Step struct {
	Asker    int // the player who writes the question
	Answerer int // the player who answers it

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
