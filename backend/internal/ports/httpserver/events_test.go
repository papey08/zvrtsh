package httpserver

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"zvrtsh/internal/app"
	"zvrtsh/internal/ports/httpserver/api"
	sessionfake "zvrtsh/internal/repo/session_repo/fake"
	userfake "zvrtsh/internal/repo/user_repo/fake"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// newLiveServer starts a real HTTP server, WebSockets need it. Cancelling the
// returned function imitates the server shutdown.
func newLiveServer(t *testing.T, events eventsConfig) (*testServer, string, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())

	cfg := app.Config{InviteTTL: time.Hour, SessionTTL: time.Hour}
	a := app.New(ctx, cfg, userfake.New(), sessionfake.New())
	router := newRouterWithEvents(ctx, a, events)
	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		cancel()
		srv.Close()
	})
	return &testServer{t: t, router: router}, "ws" + strings.TrimPrefix(srv.URL, "http") + eventsPath, cancel
}

func dial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.CloseNow() })
	return conn
}

// connect opens the socket of the user and waits until it is ready.
func connect(t *testing.T, url string, token string) *websocket.Conn {
	t.Helper()
	conn := dial(t, url)
	send(t, conn, api.EventsAuthMessage{Type: api.Auth, Token: token})
	if msg := receive(t, conn); msg.Type != api.Ready {
		t.Fatalf("first message is %q, want ready", msg.Type)
	}
	return conn
}

func send(t *testing.T, conn *websocket.Conn, msg any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, msg); err != nil {
		t.Fatal(err)
	}
}

func receive(t *testing.T, conn *websocket.Conn) api.EventsServerMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var msg api.EventsServerMessage
	if err := wsjson.Read(ctx, conn, &msg); err != nil {
		t.Fatal(err)
	}
	return msg
}

// closeStatus reads until the server closes the connection.
func closeStatus(t *testing.T, conn *websocket.Conn) websocket.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if _, _, err := conn.Read(ctx); err != nil {
			return websocket.CloseStatus(err)
		}
	}
}

func TestEvents(t *testing.T) {
	s, url, _ := newLiveServer(t, defaultEventsConfig)
	alice, bobby := s.signUp("alice"), s.signUp("bobby")
	aliceConn, bobbyConn := connect(t, url, alice), connect(t, url, bobby)

	var invite api.Invite
	s.expect(201, "POST", "/invites", bobby, api.CreateInviteRequest{StepsAmount: 2}, &invite)
	if msg := receive(t, bobbyConn); msg.Type != api.SessionUpdated || msg.Session.Status != api.Lobby || msg.Session.Version != 1 {
		t.Fatalf("unexpected event about a new lobby: %+v", msg)
	}

	s.expect(201, "POST", "/sessions", alice, api.CreateSessionRequest{InviteCode: invite.InviteCode}, nil)
	for _, conn := range []*websocket.Conn{aliceConn, bobbyConn} {
		msg := receive(t, conn)
		if msg.Type != api.SessionUpdated || msg.Session == nil || msg.Session.Id != invite.SessionId ||
			msg.Session.Version != 2 || len(msg.Session.Players) != 2 {
			t.Fatalf("unexpected event about a joined player: %+v", msg)
		}
	}

	s.expect(200, "POST", "/sessions/"+invite.SessionId+"/start", bobby, nil, nil)
	for _, conn := range []*websocket.Conn{aliceConn, bobbyConn} {
		if msg := receive(t, conn); msg.Session.Status != api.InWork || msg.Session.Version != 3 {
			t.Fatalf("unexpected event about the start: %+v", msg)
		}
	}

	s.expect(204, "POST", "/sessions/"+invite.SessionId+"/steps/0/question", alice,
		api.QuestionRequest{HiddenQuestion: "h", OpenQuestion: "o"}, nil)

	// every player sees the session from their own side
	if got := receive(t, aliceConn).Session; got.Version != 4 || !got.Progress[0].MyQuestion || got.Progress[0].QuestionsWritten != 1 {
		t.Errorf("alice's view: %+v", got)
	}
	if got := receive(t, bobbyConn).Session; got.Version != 4 || got.Progress[0].MyQuestion || got.Progress[0].QuestionsWritten != 1 {
		t.Errorf("bobby's view: %+v", got)
	}
}

func TestEventsLobbyCancelled(t *testing.T) {
	s, url, _ := newLiveServer(t, defaultEventsConfig)
	alice, bobby := s.signUp("alice"), s.signUp("bobby")

	var invite api.Invite
	s.expect(201, "POST", "/invites", bobby, api.CreateInviteRequest{StepsAmount: 2}, &invite)
	s.expect(201, "POST", "/sessions", alice, api.CreateSessionRequest{InviteCode: invite.InviteCode}, nil)

	aliceConn := connect(t, url, alice)
	s.expect(204, "DELETE", "/invites", bobby, nil, nil)
	if msg := receive(t, aliceConn); msg.Type != api.SessionDeleted || msg.SessionId == nil || *msg.SessionId != invite.SessionId {
		t.Errorf("unexpected event about the cancelled lobby: %+v", msg)
	}
}

func TestEventsAuth(t *testing.T) {
	_, url, _ := newLiveServer(t, eventsConfig{authTimeout: 200 * time.Millisecond, pingInterval: time.Minute})

	conn := dial(t, url)
	send(t, conn, api.EventsAuthMessage{Type: api.Auth, Token: "wrong"})
	if status := closeStatus(t, conn); status != websocket.StatusPolicyViolation {
		t.Errorf("wrong token: close status %v, want %v", status, websocket.StatusPolicyViolation)
	}

	conn = dial(t, url)
	send(t, conn, map[string]string{"type": "hello"})
	if status := closeStatus(t, conn); status != websocket.StatusPolicyViolation {
		t.Errorf("wrong message: close status %v, want %v", status, websocket.StatusPolicyViolation)
	}

	conn = dial(t, url)
	if status := closeStatus(t, conn); status != websocket.StatusPolicyViolation {
		t.Errorf("no auth message: close status %v, want %v", status, websocket.StatusPolicyViolation)
	}
}

func TestEventsShutdown(t *testing.T) {
	s, url, shutdown := newLiveServer(t, defaultEventsConfig)
	conn := connect(t, url, s.signUp("alice"))

	shutdown()
	if status := closeStatus(t, conn); status != websocket.StatusGoingAway {
		t.Errorf("close status %v, want %v", status, websocket.StatusGoingAway)
	}
}

func TestEventsPing(t *testing.T) {
	s, url, _ := newLiveServer(t, eventsConfig{authTimeout: time.Second, pingInterval: 50 * time.Millisecond})
	conn := connect(t, url, s.signUp("alice"))

	// a client that reads answers pings, so the connection survives several of them
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if _, _, err := conn.Read(ctx); ctx.Err() == nil {
		t.Fatalf("connection was closed: %v", err)
	}
}
