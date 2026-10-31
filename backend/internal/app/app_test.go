package app

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"
	"zvrtsh/internal/model"
	sessionfake "zvrtsh/internal/repo/session_repo/fake"
	userfake "zvrtsh/internal/repo/user_repo/fake"
)

func newTestApp(t *testing.T) *app {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := Config{InviteTTL: time.Hour, SessionTTL: time.Hour}
	a := New(ctx, cfg, userfake.New(), sessionfake.New()).(*app)
	// the same deal in every test run
	seed := uint64(0)
	a.newRand = func() *rand.Rand {
		seed++
		return rand.New(rand.NewPCG(seed, seed))
	}
	return a
}

// setClock makes the app think that it is now + shift.
func setClock(a *app, now time.Time, shift time.Duration) {
	a.sessionAppImpl.now = func() time.Time { return now.Add(shift) }
}

func register(t *testing.T, a App, name string) {
	t.Helper()
	if _, err := a.RegisterUser(context.Background(), name, "password1"); err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
}

// startGame hosts a session by the first player, lets the others join it and
// starts it. The fakes do not check users, so registration is not needed.
func startGame(t *testing.T, a *app, steps int, players ...string) string {
	t.Helper()
	ctx := context.Background()
	lobby, err := a.CreateInvite(ctx, players[0], steps)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range players[1:] {
		if _, err = a.JoinSession(ctx, p, lobby.Invite.Code); err != nil {
			t.Fatalf("%s joins: %v", p, err)
		}
	}
	if _, err = a.StartSession(ctx, lobby.Id, players[0]); err != nil {
		t.Fatal(err)
	}
	return lobby.Id
}

// newGame starts a session of alice and bobby hosted by alice.
func newGame(t *testing.T, steps int) (*app, string) {
	t.Helper()
	a := newTestApp(t)
	return a, startGame(t, a, steps, "alice", "bobby")
}

func names(n int) []string {
	result := make([]string, n)
	for i := range result {
		result[i] = fmt.Sprintf("player%d", i+1)
	}
	return result
}

func TestRegisterAndLogin(t *testing.T) {
	ctx := context.Background()
	a := newTestApp(t)

	cases := []struct{ name, password string }{
		{"abc", "password1"},        // name is too short
		{"имя_игрока", "password1"}, // not latin
		{"alice", "abc"},            // password is too short
		{"alice", "пароль123"},      // not latin
		{"alice", "has space"},      // forbidden symbol
		{"alice\n", "password1"},    // forbidden symbol
		{"this_name_is_longer_than_30_symbols", "password1"},
	}
	for _, c := range cases {
		if _, err := a.RegisterUser(ctx, c.name, c.password); !errors.Is(err, model.ErrValidation) {
			t.Errorf("RegisterUser(%q, %q) = %v, want validation error", c.name, c.password, err)
		}
	}

	register(t, a, "alice")
	if _, err := a.RegisterUser(ctx, "alice", "password1"); !errors.Is(err, model.ErrUserExists) {
		t.Errorf("duplicate registration: got %v, want ErrUserExists", err)
	}

	token, err := a.CreateToken(ctx, "alice", "password1")
	if err != nil {
		t.Fatal(err)
	}
	if name, err := a.Authenticate(ctx, token); err != nil || name != "alice" {
		t.Errorf("Authenticate = %q, %v", name, err)
	}

	if _, err = a.CreateToken(ctx, "alice", "wrong-password"); !errors.Is(err, model.ErrUnauthorized) {
		t.Errorf("wrong password: got %v", err)
	}
	if _, err = a.CreateToken(ctx, "nobody", "password1"); !errors.Is(err, model.ErrUnauthorized) {
		t.Errorf("unknown user: got %v", err)
	}
	for _, bad := range []string{"", "unknown"} {
		if _, err = a.Authenticate(ctx, bad); !errors.Is(err, model.ErrUnauthorized) {
			t.Errorf("Authenticate(%q) = %v", bad, err)
		}
	}
}

func TestLobby(t *testing.T) {
	ctx := context.Background()
	a := newTestApp(t)

	for _, steps := range []int{0, -1, maxStepsAmount + 1} {
		if _, err := a.CreateInvite(ctx, "host", steps); !errors.Is(err, model.ErrValidation) {
			t.Errorf("invite with %d steps: got %v", steps, err)
		}
	}

	lobby, err := a.CreateInvite(ctx, "host", 7)
	if err != nil {
		t.Fatal(err)
	}
	if lobby.Status() != model.StatusLobby || lobby.Host() != "host" || lobby.StepsAmount != 7 || lobby.Invite == nil {
		t.Fatalf("new lobby: %+v", lobby)
	}
	code := lobby.Invite.Code
	id := lobby.Id

	// nothing can be done in the lobby but joining and starting
	if _, err = a.StartSession(ctx, id, "host"); !errors.Is(err, model.ErrTooFewPlayers) {
		t.Errorf("start alone: got %v", err)
	}
	if err = a.MakeQuestion(ctx, id, "host", 0, "h", "o"); !errors.Is(err, model.ErrSessionNotStarted) {
		t.Errorf("question in the lobby: got %v", err)
	}
	if _, err = a.VoteFinish(ctx, id, "host", true); !errors.Is(err, model.ErrSessionNotStarted) {
		t.Errorf("finish vote in the lobby: got %v", err)
	}
	if _, err = a.JoinSession(ctx, "guest", "wrong code"); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("wrong code: got %v", err)
	}

	players := names(model.MaxPlayers)
	for i, p := range players[1:] {
		s, err := a.JoinSession(ctx, p, code)
		if err != nil {
			t.Fatalf("%s joins: %v", p, err)
		}
		if len(s.Players) != i+2 || s.Players[i+1].Name != p {
			t.Fatalf("players after %s joins: %v", p, s.PlayerNames())
		}
	}
	// joining again only leads back to the session, the host may use the code too
	for _, p := range []string{players[3], "host"} {
		if s, err := a.JoinSession(ctx, p, code); err != nil || len(s.Players) != model.MaxPlayers {
			t.Errorf("%s joins again: %d players, %v", p, len(s.Players), err)
		}
	}
	if _, err = a.JoinSession(ctx, "ninth", code); !errors.Is(err, model.ErrSessionFull) {
		t.Errorf("ninth player: got %v", err)
	}

	if _, err = a.StartSession(ctx, id, players[1]); !errors.Is(err, model.ErrNotHost) {
		t.Errorf("start by a guest: got %v", err)
	}
	if _, err = a.StartSession(ctx, id, "stranger"); !errors.Is(err, model.ErrNotParticipant) {
		t.Errorf("start by a stranger: got %v", err)
	}
	s, err := a.StartSession(ctx, id, "host")
	if err != nil {
		t.Fatal(err)
	}
	if s.Status() != model.StatusInWork || len(s.Sheets) != model.MaxPlayers || s.Invite != nil {
		t.Errorf("started session: status %v, %d sheets, invite %v", s.Status(), len(s.Sheets), s.Invite)
	}
	if _, err = a.StartSession(ctx, id, "host"); !errors.Is(err, model.ErrSessionStarted) {
		t.Errorf("second start: got %v", err)
	}
	// the code stops working
	if _, err = a.JoinSession(ctx, "late", code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("join a started session: got %v", err)
	}
}

func TestCancelLobby(t *testing.T) {
	ctx := context.Background()
	a := newTestApp(t)

	guest, stop := a.SubscribeSessions(ctx, "guest")
	defer stop()

	first, _ := a.CreateInvite(ctx, "host", 2)
	if _, err := a.JoinSession(ctx, "guest", first.Invite.Code); err != nil {
		t.Fatal(err)
	}
	<-guest // the guest has joined

	// a new invite cancels the previous lobby, its players are told about it
	second, _ := a.CreateInvite(ctx, "host", 3)
	select {
	case e := <-guest:
		if !e.Deleted || e.Session.Id != first.Id {
			t.Errorf("event about the cancelled lobby: %+v", e)
		}
	default:
		t.Error("the guest is not told about the cancelled lobby")
	}
	if _, err := a.GetSession(ctx, first.Id, "guest"); !errors.Is(err, model.ErrNoSession) {
		t.Errorf("cancelled lobby: got %v", err)
	}
	if _, err := a.JoinSession(ctx, "guest", first.Invite.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("code of the cancelled lobby: got %v", err)
	}

	// cancelling twice is fine
	for range 2 {
		if err := a.DeleteInvite(ctx, "host"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := a.GetSession(ctx, second.Id, "host"); !errors.Is(err, model.ErrNoSession) {
		t.Errorf("lobby after DeleteInvite: got %v", err)
	}

	// a started session is not cancelled, neither are the lobbies of others
	id := startGame(t, a, 1, "host", "guest")
	other, _ := a.CreateInvite(ctx, "guest", 1)
	if err := a.DeleteInvite(ctx, "host"); err != nil {
		t.Fatal(err)
	}
	for _, sid := range []string{id, other.Id} {
		if _, err := a.GetSession(ctx, sid, "guest"); err != nil {
			t.Errorf("session %s is cancelled: %v", sid, err)
		}
	}
}

func TestInviteExpiration(t *testing.T) {
	ctx := context.Background()
	a := newTestApp(t)
	now := time.Now().UTC()
	setClock(a, now, 0)

	lobby, _ := a.CreateInvite(ctx, "alice", 1)
	if want := now.Add(time.Hour); !lobby.Invite.ExpiresAt.Equal(want) {
		t.Errorf("invite expires at %v, want %v", lobby.Invite.ExpiresAt, want)
	}

	setClock(a, now, time.Hour)
	if _, err := a.JoinSession(ctx, "bobby", lobby.Invite.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("expired invite: got %v", err)
	}
}

func TestDeal(t *testing.T) {
	for n := model.MinPlayers; n <= model.MaxPlayers; n++ {
		deals := map[string]bool{}
		for seed := range uint64(50) {
			sheets := model.Deal(n, 4, rand.New(rand.NewPCG(seed, seed)))
			deals[fmt.Sprint(sheets)] = true
			if len(sheets) != n {
				t.Fatalf("%d players: %d sheets", n, len(sheets))
			}

			for step := range 4 {
				askers, answerers := map[int]bool{}, map[int]bool{}
				for i, sheet := range sheets {
					s := sheet.Steps[step]
					askers[s.Asker], answerers[s.Answerer] = true, true
					if step == 0 && s.Asker != i {
						t.Errorf("%d players: sheet %d starts with player %d", n, i, s.Asker)
					}
					// the sheet goes to somebody else for the answer and stays with
					// the one who has answered for the next question
					if s.Asker == s.Answerer {
						t.Errorf("%d players, seed %d: sheet %d stays with its asker on step %d", n, seed, i, step)
					}
					if step > 0 && s.Asker != sheet.Steps[step-1].Answerer {
						t.Errorf("%d players, seed %d: sheet %d is passed after the answer of step %d", n, seed, i, step-1)
					}
				}
				// everybody writes exactly one question and one answer
				if len(askers) != n || len(answerers) != n {
					t.Errorf("%d players: step %d has askers %v, answerers %v", n, step, askers, answerers)
				}
			}
		}
		// with two players the sheets are just swapped, with more the deal is random
		if want := n > 2; (len(deals) > 1) != want {
			t.Errorf("%d players: %d different deals", n, len(deals))
		}
	}
}

// playAll makes all moves of the session, every player asks and then answers
// on every step, and returns the session.
func playAll(t *testing.T, a *app, id string, players []string) model.Session {
	t.Helper()
	ctx := context.Background()
	s, _ := a.GetSession(ctx, id, players[0])
	for step := range s.StepsAmount {
		for _, p := range players {
			if err := a.MakeQuestion(ctx, id, p, step, p+" hidden", p+" open"); err != nil {
				t.Fatalf("step %d: %s makes question: %v", step, p, err)
			}
		}
		for _, p := range players {
			s, _ = a.GetSession(ctx, id, p)
			me, _ := s.PlayerIdx(p)
			asker := players[s.Sheets[s.AnswerSheetIdx(step, me)].Steps[step].Asker]

			open, err := a.GetQuestionToAnswer(ctx, id, p, step)
			if err != nil || open != asker+" open" {
				t.Fatalf("step %d: %s gets question %q, %v", step, p, open, err)
			}
			if err = a.MakeAnswer(ctx, id, p, step, p+" answer"); err != nil {
				t.Fatalf("step %d: %s makes answer: %v", step, p, err)
			}
		}
	}
	s, _ = a.GetSession(ctx, id, players[0])
	return s
}

func TestFullGame(t *testing.T) {
	ctx := context.Background()
	for _, n := range []int{2, 5, model.MaxPlayers} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			a := newTestApp(t)
			players := names(n)
			id := startGame(t, a, 3, players...)

			if _, err := a.GetResults(ctx, id, players[0]); !errors.Is(err, model.ErrSessionUndone) {
				t.Fatalf("results of a new session: got %v", err)
			}
			playAll(t, a, id, players)

			session, err := a.GetResults(ctx, id, players[1])
			if err != nil {
				t.Fatal(err)
			}
			for i, sheet := range session.Sheets {
				for step, s := range sheet.Steps {
					asker, answerer := players[s.Asker], players[s.Answerer]
					if s.OpenQuestion != asker+" open" || s.HiddenQuestion != asker+" hidden" || s.Answer != answerer+" answer" {
						t.Errorf("sheet %d, step %d: %+v", i, step, s)
					}
				}
			}
		})
	}
}

func TestStepOrderAndAccess(t *testing.T) {
	ctx := context.Background()
	a, id := newGame(t, 2)

	// nobody can answer before the question is asked
	if err := a.MakeAnswer(ctx, id, "bobby", 0, "x"); !errors.Is(err, model.ErrStepUnavailable) {
		t.Errorf("answer without question: got %v", err)
	}
	if _, err := a.GetQuestionToAnswer(ctx, id, "bobby", 0); !errors.Is(err, model.ErrStepUnavailable) {
		t.Errorf("read missing question: got %v", err)
	}
	// step 1 is closed until step 0 is answered
	if err := a.MakeQuestion(ctx, id, "alice", 1, "h", "o"); !errors.Is(err, model.ErrStepUnavailable) {
		t.Errorf("question of the next step: got %v", err)
	}

	if err := a.MakeQuestion(ctx, id, "alice", 0, "h", "o"); err != nil {
		t.Fatal(err)
	}
	if err := a.MakeQuestion(ctx, id, "alice", 0, "h", "o"); !errors.Is(err, model.ErrStepUnavailable) {
		t.Errorf("rewrite of the question: got %v", err)
	}
	// the sheets are passed on when everybody has written the question
	if err := a.MakeAnswer(ctx, id, "bobby", 0, "x"); !errors.Is(err, model.ErrStepUnavailable) {
		t.Errorf("answer before all questions: got %v", err)
	}
	if err := a.MakeQuestion(ctx, id, "bobby", 0, "h", "o"); err != nil {
		t.Fatal(err)
	}
	if err := a.MakeAnswer(ctx, id, "bobby", 0, "x"); err != nil {
		t.Fatal(err)
	}
	if err := a.MakeAnswer(ctx, id, "bobby", 0, "y"); !errors.Is(err, model.ErrStepUnavailable) {
		t.Errorf("rewrite of the answer: got %v", err)
	}

	// validation and access errors
	if err := a.MakeQuestion(ctx, id, "alice", 5, "h", "o"); !errors.Is(err, model.ErrValidation) {
		t.Errorf("step out of range: got %v", err)
	}
	if err := a.MakeQuestion(ctx, id, "alice", -1, "h", "o"); !errors.Is(err, model.ErrValidation) {
		t.Errorf("negative step: got %v", err)
	}
	for _, text := range []string{"", "   ", string(make([]rune, maxTextLen+1))} {
		if err := a.MakeAnswer(ctx, id, "alice", 0, text); !errors.Is(err, model.ErrValidation) {
			t.Errorf("answer %q: got %v", text, err)
		}
	}
	if _, err := a.GetSession(ctx, id, "carol"); !errors.Is(err, model.ErrNotParticipant) {
		t.Errorf("stranger reads the session: got %v", err)
	}
	if err := a.MakeQuestion(ctx, id, "carol", 0, "h", "o"); !errors.Is(err, model.ErrNotParticipant) {
		t.Errorf("stranger writes the question: got %v", err)
	}
	if _, err := a.GetSession(ctx, "no-such-session", "alice"); !errors.Is(err, model.ErrNoSession) {
		t.Errorf("unknown session: got %v", err)
	}
}

func TestReturnedSessionIsACopy(t *testing.T) {
	ctx := context.Background()
	a, id := newGame(t, 1)

	session, _ := a.GetSession(ctx, id, "alice")
	session.Sheets[0].Steps[0].OpenQuestion = "tampered"
	session.Players[0].Name = "tampered"

	if err := a.MakeQuestion(ctx, id, "alice", 0, "h", "o"); err != nil {
		t.Errorf("stored session was changed through the returned copy: %v", err)
	}
}

func TestPurgeExpiredSessions(t *testing.T) {
	ctx := context.Background()
	a, id := newGame(t, 1)

	now := time.Now().UTC()
	setClock(a, now, 30*time.Minute)
	if err := a.purgeExpired(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := a.GetSession(ctx, id, "alice"); err != nil {
		t.Fatalf("active session was purged: %v", err)
	}

	// any action prolongs the session
	if err := a.MakeQuestion(ctx, id, "alice", 0, "h", "o"); err != nil {
		t.Fatal(err)
	}
	setClock(a, now, 80*time.Minute)
	_ = a.purgeExpired(ctx)
	if _, err := a.GetSession(ctx, id, "alice"); err != nil {
		t.Fatalf("session was purged too early: %v", err)
	}

	setClock(a, now, 3*time.Hour)
	_ = a.purgeExpired(ctx)
	if _, err := a.GetSession(ctx, id, "alice"); !errors.Is(err, model.ErrNoSession) {
		t.Fatalf("expired session was not purged: %v", err)
	}
}

func TestProgressAndList(t *testing.T) {
	ctx := context.Background()
	a := newTestApp(t)
	players := []string{"alice", "bobby", "carol"}
	id := startGame(t, a, 2, players...)

	for _, p := range players {
		if err := a.MakeQuestion(ctx, id, p, 0, "h", "o"); err != nil {
			t.Fatal(err)
		}
	}
	session, _ := a.GetSession(ctx, id, "alice")
	answerer := session.Sheets[0].Steps[0].Answerer // the one who answers alice
	if err := a.MakeAnswer(ctx, id, players[answerer], 0, "a"); err != nil {
		t.Fatal(err)
	}
	session, _ = a.GetSession(ctx, id, "alice")

	if got := session.Progress(0)[0]; !got.MyQuestion || got.QuestionsWritten != 3 || got.AnswersWritten != 1 {
		t.Errorf("alice's progress: %+v", got)
	}
	if got := session.Progress(answerer)[0]; !got.MyAnswer || !got.AnswerSheetReady || got.AnswerSheetFrom != 0 {
		t.Errorf("progress of %s: %+v", players[answerer], got)
	}
	// the one who has answered on the sheet of alice writes the next question
	// on it at once, the others still answer
	if err := a.MakeQuestion(ctx, id, players[answerer], 1, "h", "o"); err != nil {
		t.Errorf("question on the answered sheet: %v", err)
	}
	other := 3 - answerer // neither alice nor the answerer
	if err := a.MakeQuestion(ctx, id, players[other], 1, "h", "o"); !errors.Is(err, model.ErrStepUnavailable) {
		t.Errorf("question before the answer: got %v", err)
	}

	// the second session of bobby becomes the most recent one
	second := startGame(t, a, 1, "dave", "bobby")
	list, _ := a.ListSessions(ctx, "bobby")
	if len(list) != 2 || list[0].Id != second || list[1].Id != id {
		t.Errorf("sessions of bobby: %+v", list)
	}
	if list, _ = a.ListSessions(ctx, "alice"); len(list) != 1 {
		t.Errorf("sessions of alice: %d, want 1", len(list))
	}
	if list, _ = a.ListSessions(ctx, "nobody"); list == nil || len(list) != 0 {
		t.Errorf("sessions of a stranger: %#v, want empty", list)
	}
}

func TestSubscribeSessions(t *testing.T) {
	ctx := context.Background()
	a := newTestApp(t)

	subscribe := func(name string) <-chan model.SessionEvent {
		ch, stop := a.SubscribeSessions(ctx, name)
		t.Cleanup(stop)
		return ch
	}
	alice, bobby, carol := subscribe("alice"), subscribe("bobby"), subscribe("carol")

	lobby, _ := a.CreateInvite(ctx, "alice", 1) // version 1
	if _, err := a.JoinSession(ctx, "bobby", lobby.Invite.Code); err != nil {
		t.Fatal(err) // version 2
	}
	if _, err := a.StartSession(ctx, lobby.Id, "alice"); err != nil {
		t.Fatal(err) // version 3
	}
	if err := a.MakeQuestion(ctx, lobby.Id, "alice", 0, "h", "o"); err != nil {
		t.Fatal(err) // version 4
	}
	// a rejected action changes nothing and sends nothing
	if err := a.MakeQuestion(ctx, lobby.Id, "alice", 0, "h", "o"); err == nil {
		t.Fatal("question was written twice")
	}

	expect := func(name string, ch <-chan model.SessionEvent, versions ...int64) {
		for _, want := range versions {
			select {
			case e := <-ch:
				if e.Session.Id != lobby.Id || e.Session.Version != want || e.Deleted {
					t.Errorf("%s got session %s of version %d, want version %d", name, e.Session.Id, e.Session.Version, want)
				}
			default:
				t.Fatalf("%s did not get the update of version %d", name, want)
			}
		}
		select {
		case e := <-ch:
			t.Errorf("%s got an unexpected update of version %d", name, e.Session.Version)
		default:
		}
	}
	expect("alice", alice, 1, 2, 3, 4)
	expect("bobby", bobby, 2, 3, 4) // bobby is a player since version 2
	expect("carol", carol)
}

func TestUnsubscribe(t *testing.T) {
	a := newTestApp(t)
	ch, stop := a.SubscribeSessions(context.Background(), "alice")
	stop()
	if _, ok := <-ch; ok {
		t.Error("channel is open after unsubscribe")
	}
	stop() // second call is harmless
}

func TestSlowSubscriberIsDropped(t *testing.T) {
	h := newHub()
	ch := h.subscribe("alice")
	event := model.SessionEvent{Session: model.Session{Players: []model.Player{{Name: "alice"}, {Name: "bobby"}}}}

	for range subscriptionBuffer + 1 {
		h.publish(event) // must not block
	}
	n := 0
	for range ch {
		n++
	}
	if n != subscriptionBuffer {
		t.Errorf("got %d updates before the channel was closed, want %d", n, subscriptionBuffer)
	}
	h.unsubscribe("alice", ch) // must not close the channel twice
}
