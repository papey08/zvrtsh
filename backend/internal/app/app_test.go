package app

import (
	"context"
	"errors"
	"testing"
	"time"
	"zvrtsh/internal/model"
	invitefake "zvrtsh/internal/repo/invite_repo/fake"
	sessionfake "zvrtsh/internal/repo/session_repo/fake"
	userfake "zvrtsh/internal/repo/user_repo/fake"
)

func newTestApp(t *testing.T) *app {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := Config{InviteTTL: time.Hour, SessionTTL: time.Hour}
	return New(ctx, cfg, userfake.New(), invitefake.New(), sessionfake.New()).(*app)
}

// setClock makes the app think that it is now + shift.
func setClock(a *app, now time.Time, shift time.Duration) {
	clock := func() time.Time { return now.Add(shift) }
	a.sessionAppImpl.now = clock
	a.userAppImpl.now = clock
}

func register(t *testing.T, a App, name string) {
	t.Helper()
	if _, err := a.RegisterUser(context.Background(), name, "password1"); err != nil {
		t.Fatalf("register %s: %v", name, err)
	}
}

// newGame registers alice and bob and starts a session created by alice.
func newGame(t *testing.T, steps int) (*app, string) {
	t.Helper()
	a := newTestApp(t)
	register(t, a, "alice")
	register(t, a, "bobby")

	invite, err := a.CreateInvite(context.Background(), "bobby", steps)
	if err != nil {
		t.Fatal(err)
	}
	session, err := a.CreateSession(context.Background(), "alice", invite.Code)
	if err != nil {
		t.Fatal(err)
	}
	return a, session.Id
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

func TestInvite(t *testing.T) {
	ctx := context.Background()
	a := newTestApp(t)
	register(t, a, "alice")
	register(t, a, "bobby")

	for _, steps := range []int{0, -1, maxStepsAmount + 1} {
		if _, err := a.CreateInvite(ctx, "bobby", steps); !errors.Is(err, model.ErrValidation) {
			t.Errorf("invite with %d steps: got %v", steps, err)
		}
	}

	invite, _ := a.CreateInvite(ctx, "bobby", 2)
	if _, err := a.CreateSession(ctx, "bobby", invite.Code); !errors.Is(err, model.ErrValidation) {
		t.Errorf("session with yourself: got %v", err)
	}
	// the invite was spent by the failed attempt
	if _, err := a.CreateSession(ctx, "alice", invite.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("spent invite: got %v", err)
	}

	// the session gets the number of steps chosen by the inviter
	invite, _ = a.CreateInvite(ctx, "bobby", 7)
	session, err := a.CreateSession(ctx, "alice", invite.Code)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if n := len(session.Sheets[0].Steps); n != 7 {
		t.Errorf("session has %d steps, want 7", n)
	}
	if _, err = a.CreateSession(ctx, "alice", invite.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("invite reuse: got %v", err)
	}

	// a new invite replaces the previous one
	first, _ := a.CreateInvite(ctx, "bobby", 2)
	second, _ := a.CreateInvite(ctx, "bobby", 2)
	if _, err = a.CreateSession(ctx, "alice", first.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("replaced invite: got %v", err)
	}
	if _, err = a.CreateSession(ctx, "alice", second.Code); err != nil {
		t.Errorf("actual invite: %v", err)
	}

	// a cancelled invite cannot be used; cancelling nothing is fine
	invite, _ = a.CreateInvite(ctx, "bobby", 2)
	for i := 0; i < 2; i++ {
		if err = a.DeleteInvite(ctx, "bobby"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = a.CreateSession(ctx, "alice", invite.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("cancelled invite: got %v", err)
	}
}

func TestFullGame(t *testing.T) {
	ctx := context.Background()
	const steps = 3
	a, id := newGame(t, steps)

	if _, err := a.GetResults(ctx, id, "alice"); !errors.Is(err, model.ErrSessionUndone) {
		t.Fatalf("results of a new session: got %v", err)
	}

	for step := 0; step < steps; step++ {
		for _, player := range []string{"alice", "bobby"} {
			if err := a.MakeQuestion(ctx, id, player, step, player+" hidden", player+" open"); err != nil {
				t.Fatalf("step %d: %s makes question: %v", step, player, err)
			}
		}
		for _, c := range []struct{ player, opponent string }{{"alice", "bobby"}, {"bobby", "alice"}} {
			open, err := a.GetOpponentQuestion(ctx, id, c.player, step)
			if err != nil || open != c.opponent+" open" {
				t.Fatalf("step %d: %s gets question %q, %v", step, c.player, open, err)
			}
			if err = a.MakeAnswer(ctx, id, c.player, step, c.player+" answer"); err != nil {
				t.Fatalf("step %d: %s makes answer: %v", step, c.player, err)
			}
		}
	}

	session, err := a.GetResults(ctx, id, "bobby")
	if err != nil {
		t.Fatal(err)
	}
	if session.Status() != model.StatusDone {
		t.Fatal("session is not done")
	}
	// the sheet of alice has questions of alice and answers of bob (even steps)
	// and the opposite on odd steps
	for step, s := range session.Sheets[0].Steps {
		wantQ, wantA := "alice", "bobby"
		if step%2 == 1 {
			wantQ, wantA = "bobby", "alice"
		}
		if s.OpenQuestion != wantQ+" open" || s.HiddenQuestion != wantQ+" hidden" || s.Answer != wantA+" answer" {
			t.Errorf("alice's sheet, step %d: %+v", step, s)
		}
	}
}

func TestStepOrderAndAccess(t *testing.T) {
	ctx := context.Background()
	a, id := newGame(t, 2)

	// nobody can answer before the opponent has asked
	if err := a.MakeAnswer(ctx, id, "bobby", 0, "x"); !errors.Is(err, model.ErrStepUnavailable) {
		t.Errorf("answer without question: got %v", err)
	}
	if _, err := a.GetOpponentQuestion(ctx, id, "bobby", 0); !errors.Is(err, model.ErrStepUnavailable) {
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
	register(t, a, "carol")
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

func TestInviteExpiration(t *testing.T) {
	ctx := context.Background()
	a := newTestApp(t)
	register(t, a, "alice")
	register(t, a, "bobby")
	now := time.Now().UTC()
	setClock(a, now, 0)

	stale, _ := a.CreateInvite(ctx, "alice", 1)
	if want := now.Add(time.Hour); !stale.ExpiresAt.Equal(want) {
		t.Errorf("invite expires at %v, want %v", stale.ExpiresAt, want)
	}

	// an expired invite is rejected even before it is purged
	setClock(a, now, time.Hour)
	if _, err := a.CreateSession(ctx, "bobby", stale.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("expired invite: got %v", err)
	}

	setClock(a, now, 0)
	_, _ = a.CreateInvite(ctx, "alice", 1)
	_, _ = a.CreateInvite(ctx, "bobby", 1)
	invites := a.userAppImpl.invites.(*invitefake.InviteRepo)
	setClock(a, now, time.Hour)
	if err := a.purgeExpiredInvites(ctx); err != nil {
		t.Fatal(err)
	}
	if n := invites.Len(); n != 0 {
		t.Errorf("%d invites left after purge", n)
	}
}

func TestProgressAndList(t *testing.T) {
	ctx := context.Background()
	a, id := newGame(t, 2)

	if err := a.MakeQuestion(ctx, id, "alice", 0, "h", "o"); err != nil {
		t.Fatal(err)
	}
	if err := a.MakeAnswer(ctx, id, "bobby", 0, "a"); err != nil {
		t.Fatal(err)
	}

	session, _ := a.GetSession(ctx, id, "alice")
	alice, _ := session.PlayerIdx("alice")
	bobby, _ := session.PlayerIdx("bobby")

	want := model.StepProgress{MyQuestion: true, OpponentAnswer: true}
	if got := session.Progress(alice)[0]; got != want {
		t.Errorf("alice's progress: %+v, want %+v", got, want)
	}
	want = model.StepProgress{OpponentQuestion: true, MyAnswer: true}
	if got := session.Progress(bobby)[0]; got != want {
		t.Errorf("bobby's progress: %+v, want %+v", got, want)
	}
	if got := session.Progress(alice)[1]; got != (model.StepProgress{}) {
		t.Errorf("progress of an untouched step: %+v", got)
	}

	// the second session of bobby becomes the most recent one
	register(t, a, "carol")
	invite, _ := a.CreateInvite(ctx, "carol", 1)
	second, err := a.CreateSession(ctx, "bobby", invite.Code)
	if err != nil {
		t.Fatal(err)
	}

	list, _ := a.ListSessions(ctx, "bobby")
	if len(list) != 2 || list[0].Id != second.Id || list[1].Id != id {
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
	for _, name := range []string{"alice", "bobby", "carol"} {
		register(t, a, name)
	}

	alice, stopAlice := a.SubscribeSessions(ctx, "alice")
	defer stopAlice()
	bobby, stopBobby := a.SubscribeSessions(ctx, "bobby")
	defer stopBobby()
	carol, stopCarol := a.SubscribeSessions(ctx, "carol")
	defer stopCarol()

	invite, _ := a.CreateInvite(ctx, "bobby", 1)
	session, err := a.CreateSession(ctx, "alice", invite.Code)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.MakeQuestion(ctx, session.Id, "alice", 0, "h", "o"); err != nil {
		t.Fatal(err)
	}
	// a rejected action changes nothing and sends nothing
	if err = a.MakeQuestion(ctx, session.Id, "alice", 0, "h", "o"); err == nil {
		t.Fatal("question was written twice")
	}

	for name, ch := range map[string]<-chan model.Session{"alice": alice, "bobby": bobby} {
		for wantVersion := int64(1); wantVersion <= 2; wantVersion++ {
			select {
			case s := <-ch:
				if s.Id != session.Id || s.Version != wantVersion {
					t.Errorf("%s got session %s of version %d, want version %d", name, s.Id, s.Version, wantVersion)
				}
			default:
				t.Fatalf("%s did not get the update of version %d", name, wantVersion)
			}
		}
		select {
		case s := <-ch:
			t.Errorf("%s got an unexpected update of version %d", name, s.Version)
		default:
		}
	}
	select {
	case <-carol:
		t.Error("a stranger got an update of the session")
	default:
	}

	// after unsubscribe the channel is closed
	stopAlice()
	if _, ok := <-alice; ok {
		t.Error("channel is open after unsubscribe")
	}
	stopAlice() // second call is harmless
}

func TestSlowSubscriberIsDropped(t *testing.T) {
	h := newHub()
	ch := h.subscribe("alice")
	session := model.Session{Sheets: [2]model.Sheet{{Owner: "alice"}, {Owner: "bobby"}}}

	for i := 0; i < subscriptionBuffer+1; i++ {
		h.publish(session) // must not block
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
