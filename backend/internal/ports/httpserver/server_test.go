package httpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
	"zvrtsh/internal/app"
	"zvrtsh/internal/ports/httpserver/api"
	sessionfake "zvrtsh/internal/repo/session_repo/fake"
	userfake "zvrtsh/internal/repo/user_repo/fake"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"
)

type testServer struct {
	t      *testing.T
	router *gin.Engine
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cfg := app.Config{InviteTTL: time.Hour, SessionTTL: time.Hour}
	a := app.New(ctx, cfg, userfake.New(), sessionfake.New())
	return &testServer{t: t, router: newRouter(ctx, a)}
}

// do sends a request and decodes the response body into out (if not nil).
func (s *testServer) do(method, path, token string, body any, out any) int {
	s.t.Helper()
	var reader bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&reader).Encode(body); err != nil {
			s.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, basePath+path, &reader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, req)

	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			s.t.Fatalf("%s %s: bad response body %q: %v", method, path, rec.Body.String(), err)
		}
	}
	return rec.Code
}

func (s *testServer) expect(want int, method, path, token string, body any, out any) {
	s.t.Helper()
	if got := s.do(method, path, token, body, out); got != want {
		s.t.Fatalf("%s %s: status %d, want %d", method, path, got, want)
	}
}

func (s *testServer) signUp(name string) string {
	s.t.Helper()
	creds := api.Credentials{Name: name, Password: "password1"}
	s.expect(http.StatusCreated, "POST", "/users", "", creds, nil)

	var token api.Token
	s.expect(http.StatusOK, "POST", "/auth/token", "", creds, &token)
	return token.Token
}

// startGame hosts a session by the first token, lets the others join and starts it.
func (s *testServer) startGame(steps int, tokens ...string) api.Session {
	s.t.Helper()
	var invite api.Invite
	s.expect(http.StatusCreated, "POST", "/invites", tokens[0], api.CreateInviteRequest{StepsAmount: steps}, &invite)
	for _, token := range tokens[1:] {
		s.expect(http.StatusCreated, "POST", "/sessions", token, api.CreateSessionRequest{InviteCode: invite.InviteCode}, nil)
	}
	var session api.Session
	s.expect(http.StatusOK, "POST", "/sessions/"+invite.SessionId+"/start", tokens[0], nil, &session)
	return session
}

func TestLobby(t *testing.T) {
	s := newTestServer(t)
	alice, bobby, carol := s.signUp("alice"), s.signUp("bobby"), s.signUp("carol")

	// a cancelled invite cannot be used
	var invite api.Invite
	s.expect(http.StatusCreated, "POST", "/invites", bobby, api.CreateInviteRequest{StepsAmount: 3}, &invite)
	s.expect(http.StatusNoContent, "DELETE", "/invites", bobby, nil, nil)
	s.expect(http.StatusNotFound, "POST", "/sessions", alice, api.CreateSessionRequest{InviteCode: invite.InviteCode}, nil)

	s.expect(http.StatusCreated, "POST", "/invites", bobby, api.CreateInviteRequest{StepsAmount: 2}, &invite)
	if invite.StepsAmount != 2 || invite.SessionId == "" {
		t.Fatalf("unexpected invite: %+v", invite)
	}
	sessionPath := "/sessions/" + invite.SessionId

	var session api.Session
	s.expect(http.StatusOK, "GET", sessionPath, bobby, nil, &session)
	if session.Status != api.Lobby || session.Host != "bobby" || session.Invite == nil || session.Invite.InviteCode != invite.InviteCode || len(session.Progress) != 0 {
		t.Fatalf("unexpected lobby: %+v", session)
	}
	s.expect(http.StatusConflict, "POST", sessionPath+"/start", bobby, nil, nil)

	for _, token := range []string{alice, carol} {
		s.expect(http.StatusCreated, "POST", "/sessions", token, api.CreateSessionRequest{InviteCode: invite.InviteCode}, &session)
	}
	if strings.Join(session.Players, ",") != "bobby,alice,carol" || session.Status != api.Lobby {
		t.Fatalf("players of the lobby: %+v", session)
	}
	s.expect(http.StatusConflict, "POST", sessionPath+"/steps/0/question", alice, api.QuestionRequest{HiddenQuestion: "h", OpenQuestion: "o"}, nil)
	s.expect(http.StatusForbidden, "POST", sessionPath+"/start", alice, nil, nil)

	session = api.Session{}
	s.expect(http.StatusOK, "POST", sessionPath+"/start", bobby, nil, &session)
	if session.Status != api.InWork || session.Invite != nil || len(session.Progress) != 2 {
		t.Fatalf("started session: %+v", session)
	}
	s.expect(http.StatusConflict, "POST", sessionPath+"/start", bobby, nil, nil)
	s.expect(http.StatusNotFound, "POST", "/sessions", s.signUp("david"), api.CreateSessionRequest{InviteCode: invite.InviteCode}, nil)
}

func TestGame(t *testing.T) {
	s := newTestServer(t)
	alice, bobby := s.signUp("alice"), s.signUp("bobby")

	session := s.startGame(2, bobby, alice)
	if session.Status != api.InWork || session.StepsAmount != 2 || len(session.Players) != 2 {
		t.Fatalf("unexpected session: %+v", session)
	}
	sessionPath := "/sessions/" + session.Id

	s.expect(http.StatusOK, "GET", sessionPath, bobby, nil, &session)
	if p := session.Progress; len(p) != 2 || p[0].MyQuestion || p[0].AnswerSheetReady || p[0].AnswerSheetFrom != "alice" {
		t.Fatalf("unexpected progress: %+v", session.Progress)
	}
	var list []api.Session
	s.expect(http.StatusOK, "GET", "/sessions", bobby, nil, &list)
	if len(list) != 1 || list[0].Id != session.Id {
		t.Fatalf("unexpected sessions list: %+v", list)
	}
	s.expect(http.StatusConflict, "GET", sessionPath+"/results", alice, nil, nil)

	for step, stepPath := range []string{sessionPath + "/steps/0", sessionPath + "/steps/1"} {
		for name, token := range map[string]string{"alice": alice, "bobby": bobby} {
			s.expect(http.StatusNoContent, "POST", stepPath+"/question", token,
				api.QuestionRequest{HiddenQuestion: name + " hidden", OpenQuestion: name + " open"}, nil)
		}
		for name, token := range map[string]string{"alice": alice, "bobby": bobby} {
			var q api.OpenQuestion
			s.expect(http.StatusOK, "GET", stepPath+"/question-to-answer", token, nil, &q)
			if q.OpenQuestion == name+" open" {
				t.Errorf("step %d: %s got their own question", step, name)
			}
			s.expect(http.StatusNoContent, "POST", stepPath+"/answer", token, api.AnswerRequest{Answer: "42"}, nil)
		}
	}

	s.expect(http.StatusOK, "GET", sessionPath, alice, nil, &session)
	if session.Status != api.Done {
		t.Errorf("session is not done: %+v", session)
	}
	for _, p := range session.Progress {
		if !p.MyQuestion || !p.MyAnswer || p.QuestionsWritten != 2 || p.AnswersWritten != 2 {
			t.Errorf("progress of a finished session: %+v", p)
		}
	}

	var results api.Results
	s.expect(http.StatusOK, "GET", sessionPath+"/results", alice, nil, &results)
	if results.Id != session.Id || len(results.Sheets) != 2 || len(results.Sheets[0].Steps) != 2 {
		t.Fatalf("unexpected results: %+v", results)
	}
	if step := results.Sheets[0].Steps[0]; step.Answer != "42" || step.Asker != "bobby" || step.Answerer != "alice" {
		t.Errorf("unexpected step: %+v", results.Sheets[0].Steps[0])
	}

	// no secrets in the results
	var raw map[string]any
	s.expect(http.StatusOK, "GET", sessionPath+"/results", alice, nil, &raw)
	if strings.Contains(toJSON(raw), alice) {
		t.Error("results contain a token")
	}
}

func TestErrors(t *testing.T) {
	s := newTestServer(t)
	alice, bobby, carol := s.signUp("alice"), s.signUp("bobby"), s.signUp("carol")

	session := s.startGame(2, bobby, alice)
	step := "/sessions/" + session.Id + "/steps/0"

	var e api.Error
	cases := []struct {
		name   string
		want   int
		method string
		path   string
		token  string
		body   any
	}{
		{"bad credentials", 401, "POST", "/auth/token", "", api.Credentials{Name: "alice", Password: "wrong"}},
		{"unknown user", 401, "POST", "/auth/token", "", api.Credentials{Name: "nobody", Password: "password1"}},
		{"duplicate user", 409, "POST", "/users", "", api.Credentials{Name: "alice", Password: "password1"}},
		{"invalid user", 400, "POST", "/users", "", api.Credentials{Name: "a", Password: "password1"}},
		{"broken json", 400, "POST", "/users", "", "not an object"},
		{"no token", 401, "POST", "/invites", "", api.CreateInviteRequest{StepsAmount: 1}},
		{"cancel without token", 401, "DELETE", "/invites", "", nil},
		{"list without token", 401, "GET", "/sessions", "", nil},
		{"wrong token", 401, "POST", "/invites", "wrong", api.CreateInviteRequest{StepsAmount: 1}},
		{"stranger", 403, "GET", "/sessions/" + session.Id, carol, nil},
		{"no session", 404, "GET", "/sessions/unknown", alice, nil},
		{"no invite", 404, "POST", "/sessions", alice, api.CreateSessionRequest{InviteCode: "x"}},
		{"wrong steps amount", 400, "POST", "/invites", alice, api.CreateInviteRequest{StepsAmount: 0}},
		{"invite without body", 400, "POST", "/invites", alice, nil},
		{"empty answer", 400, "POST", step + "/answer", alice, api.AnswerRequest{}},
		{"answer too early", 409, "POST", step + "/answer", alice, api.AnswerRequest{Answer: "a"}},
		{"question not written", 409, "GET", step + "/question-to-answer", alice, nil},
		{"step out of range", 400, "GET", "/sessions/" + session.Id + "/steps/7/question-to-answer", alice, nil},
		{"step is not a number", 400, "GET", "/sessions/" + session.Id + "/steps/x/question-to-answer", alice, nil},
		{"start by a guest", 403, "POST", "/sessions/" + session.Id + "/start", alice, nil},
		{"start of a started session", 409, "POST", "/sessions/" + session.Id + "/start", bobby, nil},
		{"unknown route", 404, "GET", "/nothing", alice, nil},
	}
	for _, c := range cases {
		e = api.Error{}
		if got := s.do(c.method, c.path, c.token, c.body, &e); got != c.want {
			t.Errorf("%s: status %d, want %d", c.name, got, c.want)
		}
		if e.Error == "" {
			t.Errorf("%s: no error message in the response", c.name)
		}
	}
}

// TestPublicRoutesMatchContract checks that exactly the operations without
// `security` in api/openapi.yaml are available without a token.
func TestPublicRoutesMatchContract(t *testing.T) {
	data, err := os.ReadFile("../../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err = yaml.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}

	public := make(map[string]bool)
	for path, methods := range spec.Paths {
		for method, op := range methods {
			if method == "parameters" {
				continue
			}
			if security, _ := op.(map[string]any)["security"].([]any); len(security) == 0 {
				public[strings.ToUpper(method)+" "+basePath+openapiToGin(path)] = true
			}
		}
	}

	if len(public) != len(publicRoutes) {
		t.Errorf("public routes: contract %v, code %v", public, publicRoutes)
	}
	for route := range public {
		if !publicRoutes[route] {
			t.Errorf("%s is public in the contract, but requires a token in the code", route)
		}
	}
}

// openapiToGin turns /sessions/{session_id} into /sessions/:session_id.
func openapiToGin(path string) string {
	var b strings.Builder
	for _, r := range path {
		switch r {
		case '{':
			b.WriteByte(':')
		case '}':
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestFinishVote(t *testing.T) {
	s := newTestServer(t)
	alice, bobby, carol := s.signUp("alice"), s.signUp("bobby"), s.signUp("carol")

	session := s.startGame(5, bobby, alice)
	votePath := "/sessions/" + session.Id + "/finish-vote"

	s.expect(http.StatusOK, "PUT", votePath, alice, nil, &session)
	if strings.Join(session.FinishVotes, ",") != "alice" {
		t.Errorf("votes after alice's vote: %v", session.FinishVotes)
	}

	s.expect(http.StatusOK, "DELETE", votePath, alice, nil, &session)
	if len(session.FinishVotes) != 0 {
		t.Errorf("vote is not withdrawn: %v", session.FinishVotes)
	}

	s.expect(http.StatusForbidden, "PUT", votePath, carol, nil, nil)
	s.expect(http.StatusNotFound, "PUT", "/sessions/unknown/finish-vote", alice, nil, nil)
	s.expect(http.StatusUnauthorized, "PUT", votePath, "", nil, nil)

	// nothing is written yet, so the game is cut to the first step
	s.expect(http.StatusOK, "PUT", votePath, alice, nil, nil)
	s.expect(http.StatusOK, "PUT", votePath, bobby, nil, &session)
	if session.StepsAmount != 1 || len(session.Progress) != 1 || len(session.FinishVotes) != 0 {
		t.Fatalf("session after the agreement: %+v", session)
	}

	step := "/sessions/" + session.Id + "/steps/0"
	for _, token := range []string{alice, bobby} {
		s.expect(http.StatusNoContent, "POST", step+"/question", token, api.QuestionRequest{HiddenQuestion: "h", OpenQuestion: "o"}, nil)
	}
	for _, token := range []string{alice, bobby} {
		s.expect(http.StatusNoContent, "POST", step+"/answer", token, api.AnswerRequest{Answer: "a"}, nil)
	}
	s.expect(http.StatusOK, "GET", "/sessions/"+session.Id+"/results", alice, nil, nil)
	s.expect(http.StatusConflict, "PUT", votePath, alice, nil, nil)
}
