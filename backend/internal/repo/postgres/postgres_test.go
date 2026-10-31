package postgres_test

import (
	"context"
	"errors"
	"math/rand/v2"
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

	if _, err = pool.Exec(ctx, `TRUNCATE "users", "sessions", "session_players", "steps"`); err != nil {
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

// newSession creates a session in the lobby hosted by the first player.
func newSession(at time.Time, steps int, players ...string) model.Session {
	s := model.Session{
		Id:          uuid.New().String(),
		StepsAmount: steps,
		Invite:      &model.Invite{Code: uuid.New().String(), ExpiresAt: at.Add(time.Hour)},
		CreatedAt:   at,
		LastActive:  at,
		Version:     1,
	}
	for _, p := range players {
		s.Players = append(s.Players, model.Player{Name: p})
	}
	return s
}

// started stores a started session of the players.
func started(t *testing.T, sessions *postgres.SessionRepo, at time.Time, steps int, players ...string) model.Session {
	t.Helper()
	ctx := context.Background()
	s := newSession(at, steps, players...)
	if err := sessions.CreateSession(ctx, s); err != nil {
		t.Fatal(err)
	}
	s, err := sessions.UpdateSession(ctx, s.Id, func(s *model.Session) error {
		s.Start(rand.New(rand.NewPCG(1, 2)))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return s
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

func TestSessions(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	createUsers(t, pool, "alice", "bobby", "carol")
	sessions := postgres.NewSessionRepo(pool)
	at := now()

	first := newSession(at, 2, "alice")
	if err := sessions.CreateSession(ctx, first); err != nil {
		t.Fatal(err)
	}
	if got, err := sessions.GetSession(ctx, first.Id); err != nil || !reflect.DeepEqual(got, first) {
		t.Fatalf("GetSession = %+v, %v, want %+v", got, err, first)
	}
	if id, err := sessions.FindByInvite(ctx, first.Invite.Code); err != nil || id != first.Id {
		t.Errorf("FindByInvite = %q, %v", id, err)
	}
	if _, err := sessions.FindByInvite(ctx, "unknown"); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("unknown invite: got %v", err)
	}

	// players join, then the session starts
	updated, err := sessions.UpdateSession(ctx, first.Id, func(s *model.Session) error {
		s.Players = append(s.Players, model.Player{Name: "bobby"}, model.Player{Name: "carol"})
		return nil
	})
	if err != nil || updated.Version != 2 {
		t.Fatalf("UpdateSession = version %d, %v", updated.Version, err)
	}
	updated, err = sessions.UpdateSession(ctx, first.Id, func(s *model.Session) error {
		s.Start(rand.New(rand.NewPCG(1, 2)))
		s.LastActive = at.Add(time.Second)
		return nil
	})
	if err != nil || updated.Version != 3 {
		t.Fatalf("UpdateSession = version %d, %v", updated.Version, err)
	}
	if got, _ := sessions.GetSession(ctx, first.Id); !reflect.DeepEqual(got, updated) {
		t.Fatalf("started session %+v, want %+v", got, updated)
	}
	if _, err = sessions.FindByInvite(ctx, first.Invite.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("invite of a started session: got %v", err)
	}

	// a question and then an answer
	updated, err = sessions.UpdateSession(ctx, first.Id, func(s *model.Session) error {
		s.Sheets[1].Steps[0].HiddenQuestion, s.Sheets[1].Steps[0].OpenQuestion = "Что я ел?", "Что?"
		s.LastActive = at.Add(2 * time.Second)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err = sessions.UpdateSession(ctx, first.Id, func(s *model.Session) error {
		if s.Sheets[1].Steps[0].Status() != model.StepAnswering {
			t.Errorf("update sees an old state: %+v", s.Sheets[1].Steps[0])
		}
		s.Sheets[1].Steps[0].Answer = "Пуговицы"
		s.Players[2].FinishVote = true
		s.LastActive = at.Add(3 * time.Second)
		return nil
	})
	if err != nil || updated.Version != 5 {
		t.Fatalf("UpdateSession = version %d, %v", updated.Version, err)
	}
	if got, _ := sessions.GetSession(ctx, first.Id); !reflect.DeepEqual(got, updated) {
		t.Errorf("stored session %+v, want %+v", got, updated)
	}

	// a failed update changes nothing
	errRule := errors.New("against the rules")
	if _, err = sessions.UpdateSession(ctx, first.Id, func(s *model.Session) error {
		s.Sheets[0].Steps[1].OpenQuestion = "o"
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

	second := newSession(at.Add(time.Minute), 1, "carol", "bobby")
	if err = sessions.CreateSession(ctx, second); err != nil {
		t.Fatal(err)
	}
	for user, want := range map[string][]string{
		"bobby":  {second.Id, first.Id}, // the most recently active first
		"alice":  {first.Id},
		"carol":  {second.Id, first.Id},
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

	// a session is deleted only if check allows
	if _, err = sessions.DeleteSession(ctx, second.Id, func(model.Session) error { return errRule }); !errors.Is(err, errRule) {
		t.Errorf("forbidden delete: got %v", err)
	}
	deleted, err := sessions.DeleteSession(ctx, second.Id, func(model.Session) error { return nil })
	if err != nil || !reflect.DeepEqual(deleted, second) {
		t.Errorf("DeleteSession = %+v, %v", deleted, err)
	}
	if _, err = sessions.GetSession(ctx, second.Id); !errors.Is(err, model.ErrNoSession) {
		t.Errorf("deleted session: got %v", err)
	}

	// old sessions are deleted with their players and steps
	third := started(t, sessions, at.Add(time.Minute), 1, "carol", "bobby")
	if err = sessions.DeleteSessionsBefore(ctx, at.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err = sessions.GetSession(ctx, first.Id); !errors.Is(err, model.ErrNoSession) {
		t.Errorf("old session was not deleted: %v", err)
	}
	if _, err = sessions.GetSession(ctx, third.Id); err != nil {
		t.Errorf("fresh session was deleted: %v", err)
	}
	var steps, players int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM "steps"), (SELECT count(*) FROM "session_players")`).Scan(&steps, &players); err != nil || steps != 2 || players != 2 {
		t.Errorf("%d steps and %d players left, want 2 and 2, %v", steps, players, err)
	}
}

// TestConcurrentUpdates checks that updates of a session wait for each other:
// only one of the players' simultaneous attempts to write a step succeeds.
func TestConcurrentUpdates(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	createUsers(t, pool, "alice", "bobby")
	sessions := postgres.NewSessionRepo(pool)

	session := started(t, sessions, now(), 1, "alice", "bobby")

	const attempts = 20
	var (
		wg        sync.WaitGroup
		mu        sync.Mutex
		succeeded int
	)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := sessions.UpdateSession(ctx, session.Id, func(s *model.Session) error {
				if s.Sheets[0].Steps[0].Status() != model.StepWaiting {
					return model.ErrStepUnavailable
				}
				s.Sheets[0].Steps[0].HiddenQuestion, s.Sheets[0].Steps[0].OpenQuestion = "h", "o"
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
	if got, _ := sessions.GetSession(ctx, session.Id); got.Version != 3 {
		t.Errorf("version %d, want 3", got.Version)
	}
}

// TestGame plays a whole game of three players through the application on top of Postgres.
func TestGame(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	pool := testPool(t)
	a := app.New(ctx, app.Config{InviteTTL: time.Hour, SessionTTL: time.Hour},
		postgres.NewUserRepo(pool), postgres.NewSessionRepo(pool))

	players := []string{"alice", "bobby", "carol"}
	for _, name := range players {
		if _, err := a.RegisterUser(ctx, name, "password1"); err != nil {
			t.Fatal(err)
		}
	}
	events, unsubscribe := a.SubscribeSessions(ctx, "bobby")
	defer unsubscribe()

	lobby, err := a.CreateInvite(ctx, "bobby", 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"alice", "carol"} {
		if _, err = a.JoinSession(ctx, p, lobby.Invite.Code); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = a.StartSession(ctx, lobby.Id, "bobby"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.JoinSession(ctx, "alice", lobby.Invite.Code); !errors.Is(err, model.ErrNoInviteCode) {
		t.Errorf("invite of a started session: %v", err)
	}

	for step := range 2 {
		for _, player := range players {
			if err = a.MakeQuestion(ctx, lobby.Id, player, step, player+" спрашивает", "Что?"); err != nil {
				t.Fatalf("step %d, question of %s: %v", step, player, err)
			}
		}
		for _, player := range players {
			if err = a.MakeAnswer(ctx, lobby.Id, player, step, player+" отвечает"); err != nil {
				t.Fatalf("step %d, answer of %s: %v", step, player, err)
			}
		}
	}

	results, err := a.GetResults(ctx, lobby.Id, "alice")
	if err != nil {
		t.Fatal(err)
	}
	names := results.PlayerNames()
	for _, sheet := range results.Sheets {
		for _, step := range sheet.Steps {
			if step.HiddenQuestion != names[step.Asker]+" спрашивает" || step.Answer != names[step.Answerer]+" отвечает" {
				t.Errorf("unexpected step %+v", step)
			}
		}
	}
	// creation, 2 players joined, start and 12 actions
	if results.Version != 16 {
		t.Errorf("version %d, want 16", results.Version)
	}

	var last int64
	for i := range 16 {
		select {
		case e := <-events:
			last = max(last, e.Session.Version)
		default:
			t.Fatalf("got only %d events of 16", i)
		}
	}
	if last != results.Version {
		t.Errorf("the last event has version %d, want %d", last, results.Version)
	}
}

// TestFinishVotes checks that votes and the shortened session are stored.
func TestFinishVotes(t *testing.T) {
	ctx := context.Background()
	pool := testPool(t)
	createUsers(t, pool, "alice", "bobby", "carol")
	sessions := postgres.NewSessionRepo(pool)
	session := started(t, sessions, now(), 5, "bobby", "alice", "carol")

	shorten := func(s *model.Session) error {
		for i := range s.Sheets {
			s.Sheets[i].Steps[0].HiddenQuestion, s.Sheets[i].Steps[0].OpenQuestion = "h", "o"
		}
		s.Players[1].FinishVote = true
		s.Shorten(1)
		return nil
	}
	agreed, err := sessions.UpdateSession(ctx, session.Id, shorten)
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := sessions.GetSession(ctx, session.Id)
	if !reflect.DeepEqual(stored, agreed) || stored.StepsAmount != 1 || len(stored.Sheets[2].Steps) != 1 || !stored.Players[1].FinishVote {
		t.Errorf("stored session after the agreement %+v, want %+v", stored, agreed)
	}
	if list, _ := sessions.ListSessions(ctx, "alice"); len(list) != 1 || !reflect.DeepEqual(list[0], agreed) {
		t.Errorf("listed session %+v, want %+v", list, agreed)
	}
	var steps int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM "steps"`).Scan(&steps); err != nil || steps != 3 {
		t.Errorf("%d steps are stored, want 3, %v", steps, err)
	}
}
