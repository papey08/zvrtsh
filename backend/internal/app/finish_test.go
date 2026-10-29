package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"zvrtsh/internal/model"
)

// The players of the games in this file; alice has created the session (sheet 0).
var players = [2]string{"alice", "bobby"}

// move is one action of a player: a question or an answer for the step.
type move struct {
	player   string
	question bool
	step     int
}

func (m move) String() string {
	kind := "a"
	if m.question {
		kind = "q"
	}
	return fmt.Sprintf("%s:%s%d", m.player, kind, m.step)
}

func (m move) apply(ctx context.Context, a *app, id string) error {
	if m.question {
		return a.MakeQuestion(ctx, id, m.player, m.step, m.String()+" hidden", m.String()+" open")
	}
	return a.MakeAnswer(ctx, id, m.player, m.step, m.String())
}

// newQuickGame starts a session without registration: the fakes do not check users.
func newQuickGame(t *testing.T, steps int) (*app, string) {
	t.Helper()
	a := newTestApp(t)
	invite, err := a.CreateInvite(context.Background(), players[1], steps)
	if err != nil {
		t.Fatal(err)
	}
	session, err := a.CreateSession(context.Background(), players[0], invite.Code)
	if err != nil {
		t.Fatal(err)
	}
	return a, session.Id
}

// replay starts a game and makes the moves; ok is false if a move is not allowed.
func replay(t *testing.T, steps int, moves []move) (a *app, id string, ok bool) {
	t.Helper()
	a, id = newQuickGame(t, steps)
	for _, m := range moves {
		if err := m.apply(context.Background(), a, id); err != nil {
			return a, id, false
		}
	}
	return a, id, true
}

// candidates are the only moves a player can try next: the first question
// not written yet and the first answer not given yet.
func candidates(s model.Session) []move {
	var result []move
	for _, player := range players {
		me, _ := s.PlayerIdx(player)
		progress := s.Progress(me)
		for step, p := range progress {
			if !p.MyQuestion {
				result = append(result, move{player, true, step})
				break
			}
		}
		for step, p := range progress {
			if !p.MyAnswer {
				result = append(result, move{player, false, step})
				break
			}
		}
	}
	return result
}

// playToEnd makes allowed moves until the session is done and returns how many
// questions every player had to write.
func playToEnd(t *testing.T, a *app, id string) map[string]int {
	t.Helper()
	ctx := context.Background()
	questions := map[string]int{}
	for {
		s, err := a.GetSession(ctx, id, players[0])
		if err != nil {
			t.Fatal(err)
		}
		if s.Status() == model.StatusDone {
			return questions
		}
		moved := false
		for _, m := range candidates(s) {
			if m.apply(ctx, a, id) == nil {
				if m.question {
					questions[m.player]++
				}
				moved = true
				break
			}
		}
		if !moved {
			t.Fatalf("the game is stuck: %+v", s.Progress(0))
		}
	}
}

func vote(t *testing.T, a *app, id string, player string, value bool) model.Session {
	t.Helper()
	s, err := a.VoteFinish(context.Background(), id, player, value)
	if err != nil {
		t.Fatalf("vote of %s: %v", player, err)
	}
	return s
}

// TestFinishScenarios checks the situations from the description of the feature.
func TestFinishScenarios(t *testing.T) {
	ctx := context.Background()
	// the first step is fully played in every scenario
	firstStep := []move{{"alice", true, 0}, {"bobby", true, 0}, {"alice", false, 0}, {"bobby", false, 0}}

	cases := []struct {
		name      string
		moves     []move
		wantSteps int
		// questions each player still writes after the agreement
		wantQuestions map[string]int
	}{
		{
			name:          "both are writing questions",
			moves:         firstStep,
			wantSteps:     2,
			wantQuestions: map[string]int{"alice": 1, "bobby": 1},
		},
		{
			name:          "player 1 has written the question and waits for player 2",
			moves:         append(append([]move{}, firstStep...), move{"alice", true, 1}),
			wantSteps:     2,
			wantQuestions: map[string]int{"bobby": 1},
		},
		{
			name:          "player 1 waits while player 2 still answers the previous step",
			moves:         append(append([]move{}, firstStep[:3]...), move{"alice", true, 1}),
			wantSteps:     2,
			wantQuestions: map[string]int{"bobby": 1},
		},
		{
			name:          "both are writing answers",
			moves:         append(append([]move{}, firstStep...), move{"alice", true, 1}, move{"bobby", true, 1}),
			wantSteps:     2,
			wantQuestions: map[string]int{},
		},
		{
			name:          "one has answered, the other is answering",
			moves:         append(append([]move{}, firstStep...), move{"alice", true, 1}, move{"bobby", true, 1}, move{"alice", false, 1}),
			wantSteps:     2,
			wantQuestions: map[string]int{},
		},
		{
			name:          "nothing is written yet",
			moves:         nil,
			wantSteps:     1,
			wantQuestions: map[string]int{"alice": 1, "bobby": 1},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a, id, ok := replay(t, 5, c.moves)
			if !ok {
				t.Fatal("the moves of the case are not allowed")
			}

			// one vote changes nothing but the vote
			s := vote(t, a, id, "alice", true)
			if len(s.Sheets[0].Steps) != 5 || s.FinishVotes != [2]bool{true, false} {
				t.Fatalf("after the first vote: %d steps, votes %v", len(s.Sheets[0].Steps), s.FinishVotes)
			}

			s = vote(t, a, id, "bobby", true)
			if n := len(s.Sheets[0].Steps); n != c.wantSteps {
				t.Errorf("session has %d steps after the agreement, want %d", n, c.wantSteps)
			}
			if s.FinishVotes != [2]bool{} {
				t.Errorf("votes are not reset: %v", s.FinishVotes)
			}

			if got := playToEnd(t, a, id); fmt.Sprint(got) != fmt.Sprint(c.wantQuestions) {
				t.Errorf("questions written after the agreement: %v, want %v", got, c.wantQuestions)
			}
			if _, err := a.GetResults(ctx, id, "bobby"); err != nil {
				t.Errorf("results: %v", err)
			}
		})
	}
}

// TestFinishInEveryState agrees to finish in every reachable state of a game,
// in both orders of votes, and checks that nothing written is lost, the game
// can be completed and nobody has to write more than one question.
func TestFinishInEveryState(t *testing.T) {
	const steps = 3
	ctx := context.Background()
	seen := map[string]bool{}
	var states int

	var explore func(moves []move)
	explore = func(moves []move) {
		a, id, ok := replay(t, steps, moves)
		if !ok {
			return
		}
		before, _ := a.GetSession(ctx, id, players[0])
		key := fmt.Sprint(before.Progress(0))
		if seen[key] {
			return
		}
		seen[key] = true
		if before.Status() == model.StatusDone {
			return
		}
		states++

		for _, first := range []int{0, 1} {
			a, id, _ := replay(t, steps, moves)
			vote(t, a, id, players[first], true)
			after := vote(t, a, id, players[1-first], true)

			n := len(after.Sheets[0].Steps)
			if n < 1 || n > steps {
				t.Fatalf("%v: %d steps after the agreement", moves, n)
			}
			for sheet := range before.Sheets {
				for i, step := range before.Sheets[sheet].Steps {
					if step.Status() != model.StepWaiting && (i >= n || after.Sheets[sheet].Steps[i] != step) {
						t.Errorf("%v: step %d of sheet %d is lost", moves, i, sheet)
					}
				}
			}
			for player, q := range playToEnd(t, a, id) {
				if q > 1 {
					t.Errorf("%v: %s has to write %d more questions", moves, player, q)
				}
			}
		}

		for _, m := range candidates(before) {
			explore(append(append([]move{}, moves...), m))
		}
	}
	explore(nil)

	if states < 10 {
		t.Fatalf("only %d states were checked", states)
	}
	t.Logf("checked %d states of a game of %d steps", states, steps)
}

func TestFinishVoteRules(t *testing.T) {
	ctx := context.Background()
	a, id := newQuickGame(t, 2)

	// a withdrawn vote does not count
	vote(t, a, id, "alice", true)
	if s := vote(t, a, id, "alice", false); s.FinishVotes != [2]bool{} {
		t.Errorf("vote is not withdrawn: %v", s.FinishVotes)
	}
	if s := vote(t, a, id, "bobby", true); len(s.Sheets[0].Steps) != 2 {
		t.Errorf("session was shortened by one vote: %d steps", len(s.Sheets[0].Steps))
	}

	if _, err := a.VoteFinish(ctx, id, "carol", true); !errors.Is(err, model.ErrNotParticipant) {
		t.Errorf("vote of a stranger: got %v", err)
	}
	if _, err := a.VoteFinish(ctx, "no-such-session", "alice", true); !errors.Is(err, model.ErrNoSession) {
		t.Errorf("vote in an unknown session: got %v", err)
	}

	// the players are notified about the agreement
	events, unsubscribe := a.SubscribeSessions(ctx, "bobby")
	defer unsubscribe()
	vote(t, a, id, "alice", true)
	select {
	case s := <-events:
		if len(s.Sheets[0].Steps) != 1 {
			t.Errorf("event about the agreement has %d steps", len(s.Sheets[0].Steps))
		}
	default:
		t.Error("no event about the agreement")
	}

	playToEnd(t, a, id)
	if _, err := a.VoteFinish(ctx, id, "alice", true); !errors.Is(err, model.ErrSessionDone) {
		t.Errorf("vote in a finished session: got %v", err)
	}
}
