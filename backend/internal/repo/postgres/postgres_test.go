package postgres_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
	"zvrtsh/internal/app"
	"zvrtsh/internal/model"
	"zvrtsh/internal/repo/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to the database from TEST_DATABASE_URL with all migrations
// applied (see `make test-back`) and empties it. Without the variable the test is skipped.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set, run the test with `make test-back`")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := postgres.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	if _, err = pool.Exec(ctx, `TRUNCATE "users", "invites", "sessions", "steps"`); err != nil {
		t.Fatal(err)
	}
	return pool
}

func createUsers(t *testing.T, pool *pgxpool.Pool, names ...string) {
	t.Helper()
	users := postgres.NewUserRepo(pool)
	for _, name := range names {
		if err := users.CreateUser(context.Background(), model.User{Name: name, EncodedPassword: "hash", Token: "token-" + name}); err != nil {
			t.Fatal(err)
		}
	}
}

// now has the precision of Postgres, so stored times are equal to the saved ones.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}

func newSession(player1, player2 string, steps int, at time.Time) model.Session {
	return model.Session{
		Id: uuid.New().String(),
		Sheets: [2]model.Sheet{
			{Owner: player1, Steps: make([]model.Step, steps)},
			{Owner: player2, Steps: make([]model.Step, steps)},
		},
		CreatedAt:  at,
		LastActive: at,
		Version:    1,
	}
}

func TestUsers(t *testing.T) {
	ctx := context.Background()
	users := postgres.NewUserRepo(testPool(t))

	alice := model.User{Name: "alice", EncodedPassword: "hash", Token: "secret"}
	if err := users.CreateUser(ctx, alice); err != nil {
		t.Fatal(err)
	}
	if err := users.CreateUser(ctx, alice); !errors.Is(err, model.ErrUserExists) {
		t.Errorf("duplicate user: got %v", err)
	}

	if got, err := users.GetUser(ctx, "alice"); err != nil || got != alice {
		t.Errorf("GetUser = %+v, %v", got, err)
	}
	if got, err := users.GetUserByToken(ctx, "secret"); err != nil || got != alice {
		t.Errorf("GetUserByToken = %+v, %v", got, err)
	}
	if _, err := users.GetUser(ctx, "nobody"); !errors.Is(err, model.ErrUserNotFound) {
		t.Errorf("unknown user: got %v", err)
	}
	if _, err := users.GetUserByToken(ctx, "wrong"); !errors.Is(err, model.ErrUserNotFound) {
		t.Errorf("unknown token: got %v", err)
	}
}

func TestInvites(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	createUsers(t, pool, "alice", "bobby")
	invites := postgres.NewInviteRepo(pool)
	at := now()

	first := model.Invite{Code: "first", UserName: "alice", StepsAmount: 3, CreationDate: at, ExpiresAt: at.Add(time.Hour)}
	second := model.Invite{Code: "second", UserName: "alice", StepsAmount: 7, CreationDate: at, ExpiresAt: at.Add(time.Hour)}
	if err := invites.CreateInvite(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := invites.CreateInvite(ctx, second); err != nil {
		t.Fatal(err)
	}

	// the second invite of the user replaces the first one
	if _, err := invites.TakeInvite(ctx, "first"); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("replaced invite: got %v", err)
	}
	if got, err := invites.TakeInvite(ctx, "second"); err != nil || got != second {
		t.Errorf("TakeInvite = %+v, %v, want %+v", got, err, second)
	}
	if _, err := invites.TakeInvite(ctx, "second"); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("invite taken twice: got %v", err)
	}

	expiring := model.Invite{Code: "expiring", UserName: "bobby", StepsAmount: 1, CreationDate: at, ExpiresAt: at.Add(time.Minute)}
	if err := invites.CreateInvite(ctx, expiring); err != nil {
		t.Fatal(err)
	}
	if err := invites.DeleteExpiredInvites(ctx, at.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := invites.DeleteExpiredInvites(ctx, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := invites.TakeInvite(ctx, "expiring"); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("expired invite was not deleted: %v", err)
	}

	cancelled := model.Invite{Code: "cancelled", UserName: "bobby", StepsAmount: 1, CreationDate: at, ExpiresAt: at.Add(time.Hour)}
	if err := invites.CreateInvite(ctx, cancelled); err != nil {
		t.Fatal(err)
	}
	if err := invites.DeleteInvite(ctx, "bobby"); err != nil {
		t.Fatal(err)
	}
	if err := invites.DeleteInvite(ctx, "bobby"); err != nil {
		t.Errorf("deleting a missing invite: %v", err)
	}
	if _, err := invites.TakeInvite(ctx, "cancelled"); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("cancelled invite: got %v", err)
	}

	if err := invites.CreateInvite(ctx, model.Invite{Code: "x", UserName: "nobody", StepsAmount: 1, CreationDate: at, ExpiresAt: at}); !errors.Is(err, model.ErrDatabase) {
		t.Errorf("invite of an unknown user: got %v", err)
	}
}

func TestSessions(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	createUsers(t, pool, "alice", "bobby", "carol")
	sessions := postgres.NewSessionRepo(pool)
	at := now()

	first := newSession("alice", "bobby", 2, at)
	if err := sessions.CreateSession(ctx, first); err != nil {
		t.Fatal(err)
	}
	if got, err := sessions.GetSession(ctx, first.Id); err != nil || !reflect.DeepEqual(got, first) {
		t.Fatalf("GetSession = %+v, %v, want %+v", got, err, first)
	}

	// a question and then an answer
	updated, err := sessions.UpdateSession(ctx, first.Id, func(s *model.Session) error {
		s.Sheets[1].Steps[0] = model.Step{HiddenQuestion: "Что я ел?", OpenQuestion: "Что?"}
		s.LastActive = at.Add(time.Second)
		return nil
	})
	if err != nil || updated.Version != 2 {
		t.Fatalf("UpdateSession = version %d, %v", updated.Version, err)
	}
	updated, err = sessions.UpdateSession(ctx, first.Id, func(s *model.Session) error {
		if s.Sheets[1].Steps[0].Status() != model.StepAnswering {
			t.Errorf("update sees an old state: %+v", s.Sheets[1].Steps[0])
		}
		s.Sheets[1].Steps[0].Answer = "Пуговицы"
		s.LastActive = at.Add(2 * time.Second)
		return nil
	})
	if err != nil || updated.Version != 3 {
		t.Fatalf("UpdateSession = version %d, %v", updated.Version, err)
	}
	if got, _ := sessions.GetSession(ctx, first.Id); !reflect.DeepEqual(got, updated) {
		t.Errorf("stored session %+v, want %+v", got, updated)
	}

	// a failed update changes nothing
	errRule := errors.New("against the rules")
	if _, err = sessions.UpdateSession(ctx, first.Id, func(s *model.Session) error {
		s.Sheets[0].Steps[1] = model.Step{HiddenQuestion: "h", OpenQuestion: "o"}
		return errRule
	}); !errors.Is(err, errRule) {
		t.Errorf("failed update: got %v", err)
	}
	if got, _ := sessions.GetSession(ctx, first.Id); !reflect.DeepEqual(got, updated) {
		t.Errorf("failed update has changed the session: %+v", got)
	}

	for _, id := range []string{"not-a-uuid", uuid.New().String()} {
		if _, err = sessions.GetSession(ctx, id); !errors.Is(err, model.ErrNoSession) {
			t.Errorf("GetSession(%q): got %v", id, err)
		}
		if _, err = sessions.UpdateSession(ctx, id, func(*model.Session) error { return nil }); !errors.Is(err, model.ErrNoSession) {
			t.Errorf("UpdateSession(%q): got %v", id, err)
		}
	}

	second := newSession("carol", "bobby", 1, at.Add(time.Minute))
	if err = sessions.CreateSession(ctx, second); err != nil {
		t.Fatal(err)
	}
	for user, want := range map[string][]string{
		"bobby":  {second.Id, first.Id}, // the most recently active first
		"alice":  {first.Id},
		"carol":  {second.Id},
		"nobody": {},
	} {
		list, err := sessions.ListSessions(ctx, user)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]string, len(list))
		for i, s := range list {
			ids[i] = s.Id
		}
		if !reflect.DeepEqual(ids, want) {
			t.Errorf("sessions of %s: %v, want %v", user, ids, want)
		}
		if user == "alice" && !reflect.DeepEqual(list[0], updated) {
			t.Errorf("listed session %+v, want %+v", list[0], updated)
		}
	}

	// old sessions are deleted with their steps
	if err = sessions.DeleteSessionsBefore(ctx, at.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = sessions.GetSession(ctx, first.Id); !errors.Is(err, model.ErrNoSession) {
		t.Errorf("old session was not deleted: %v", err)
	}
	if _, err = sessions.GetSession(ctx, second.Id); err != nil {
		t.Errorf("fresh session was deleted: %v", err)
	}
	var steps int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM "steps"`).Scan(&steps); err != nil || steps != 0 {
		t.Errorf("%d steps left after the session was deleted, %v", steps, err)
	}
}

// TestConcurrentUpdates checks that updates of a session wait for each other:
// only one of the players' simultaneous attempts to write a step succeeds.
func TestConcurrentUpdates(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	createUsers(t, pool, "alice", "bobby")
	sessions := postgres.NewSessionRepo(pool)

	session := newSession("alice", "bobby", 1, now())
	if err := sessions.CreateSession(ctx, session); err != nil {
		t.Fatal(err)
	}

	const attempts = 20
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
	)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := sessions.UpdateSession(ctx, session.Id, func(s *model.Session) error {
				if s.Sheets[0].Steps[0].Status() != model.StepWaiting {
					return model.ErrStepUnavailable
				}
				s.Sheets[0].Steps[0] = model.Step{HiddenQuestion: "h", OpenQuestion: "o"}
				return nil
			})
			if err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			} else if !errors.Is(err, model.ErrStepUnavailable) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if succeeded != 1 {
		t.Errorf("%d updates succeeded, want 1", succeeded)
	}
	if got, _ := sessions.GetSession(ctx, session.Id); got.Version != 2 {
		t.Errorf("version %d, want 2", got.Version)
	}
}

// TestGame plays a whole game through the application on top of Postgres.
func TestGame(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := testPool(t)
	a := app.New(ctx, app.Config{InviteTTL: time.Hour, SessionTTL: time.Hour},
		postgres.NewUserRepo(pool), postgres.NewInviteRepo(pool), postgres.NewSessionRepo(pool))

	for _, name := range []string{"alice", "bobby"} {
		if _, err := a.RegisterUser(ctx, name, "password1"); err != nil {
			t.Fatal(err)
		}
	}
	events, unsubscribe := a.SubscribeSessions(ctx, "bobby")
	defer unsubscribe()

	invite, err := a.CreateInvite(ctx, "bobby", 2)
	if err != nil {
		t.Fatal(err)
	}
	session, err := a.CreateSession(ctx, "alice", invite.Code)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.CreateSession(ctx, "alice", invite.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("invite used twice: %v", err)
	}

	for step := 0; step < 2; step++ {
		for _, player := range []string{"alice", "bobby"} {
			if err = a.MakeQuestion(ctx, session.Id, player, step, player+" спрашивает", "Что?"); err != nil {
				t.Fatalf("step %d, question of %s: %v", step, player, err)
			}
		}
		for _, player := range []string{"alice", "bobby"} {
			if err = a.MakeAnswer(ctx, session.Id, player, step, player+" отвечает"); err != nil {
				t.Fatalf("step %d, answer of %s: %v", step, player, err)
			}
		}
	}

	results, err := a.GetResults(ctx, session.Id, "alice")
	if err != nil {
		t.Fatal(err)
	}
	if want := (model.Step{HiddenQuestion: "bobby спрашивает", OpenQuestion: "Что?", Answer: "alice отвечает"}); results.Sheets[0].Steps[1] != want {
		t.Errorf("step 2 of alice's sheet: %+v, want %+v", results.Sheets[0].Steps[1], want)
	}
	// creation and 8 actions
	if results.Version != 9 {
		t.Errorf("version %d, want 9", results.Version)
	}

	var last int64
	for i := 0; i < 9; i++ {
		select {
		case s := <-events:
			last = max(last, s.Version)
		default:
			t.Fatalf("got only %d events of 9", i)
		}
	}
	if last != results.Version {
		t.Errorf("the last event has version %d, want %d", last, results.Version)
	}
}

// TestFinishVotes checks that votes and the shortened session are stored.
func TestFinishVotes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := testPool(t)
	createUsers(t, pool, "alice", "bobby")
	sessions := postgres.NewSessionRepo(pool)
	a := app.New(ctx, app.Config{InviteTTL: time.Hour, SessionTTL: time.Hour},
		postgres.NewUserRepo(pool), postgres.NewInviteRepo(pool), sessions)

	invite, err := a.CreateInvite(ctx, "bobby", 5)
	if err != nil {
		t.Fatal(err)
	}
	session, err := a.CreateSession(ctx, "alice", invite.Code)
	if err != nil {
		t.Fatal(err)
	}
	for _, player := range []string{"alice", "bobby"} {
		if err = a.MakeQuestion(ctx, session.Id, player, 0, "h", "o"); err != nil {
			t.Fatal(err)
		}
	}

	if _, err = a.VoteFinish(ctx, session.Id, "bobby", true); err != nil {
		t.Fatal(err)
	}
	stored, _ := sessions.GetSession(ctx, session.Id)
	if stored.FinishVotes != [2]bool{false, true} || len(stored.Sheets[0].Steps) != 5 {
		t.Errorf("after one vote: votes %v, %d steps", stored.FinishVotes, len(stored.Sheets[0].Steps))
	}

	agreed, err := a.VoteFinish(ctx, session.Id, "alice", true)
	if err != nil {
		t.Fatal(err)
	}
	stored, _ = sessions.GetSession(ctx, session.Id)
	if !reflect.DeepEqual(stored, agreed) || len(stored.Sheets[0].Steps) != 1 || stored.FinishVotes != [2]bool{} {
		t.Errorf("stored session after the agreement %+v, want %+v", stored, agreed)
	}
	if list, _ := sessions.ListSessions(ctx, "alice"); len(list) != 1 || !reflect.DeepEqual(list[0], agreed) {
		t.Errorf("listed session %+v, want %+v", list, agreed)
	}
}
