package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"zvrtsh/internal/ports/httpserver"
)

var (
	ErrClient            = errors.New("client error")
	ErrUserNotAuthorized = errors.New("not authorized user")
	ErrNoSessionCode     = errors.New("no session code")
)

func (a *App) RegisterUser(_ context.Context, name string, password string) error {
	reqBody, _ := json.Marshal(httpserver.RegisterUserRequest{
		Name:     name,
		Password: password,
	})

	req, _ := http.NewRequest(http.MethodPost, a.baseUrl+"user", bytes.NewReader(reqBody))

	respData, err := a.makeRequest(req)
	if err != nil {
		return err
	}

	if respData.Data != nil {
		a.name = name
		a.secretCode = respData.Data.(httpserver.UserData).SecretCode
		return nil
	} else if respData.Err != nil {
		return errors.New(*respData.Err)
	} else {
		return ErrClient
	}
}

func (a *App) LoginUser(_ context.Context, name string, password string) error {
	reqBody, _ := json.Marshal(httpserver.GetUserSecretCodeRequest{
		Password: password,
	})
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%sinvite/%s", a.baseUrl, name), bytes.NewReader(reqBody))

	respData, err := a.makeRequest(req)
	if err != nil {
		return err
	}

	if respData.Data != nil {
		a.name = name
		a.secretCode = respData.Data.(httpserver.UserSecretCodeData).SecretCode
		return nil
	} else if respData.Err != nil {
		return errors.New(*respData.Err)
	} else {
		return ErrClient
	}
}

func (a *App) GetUserInviteCode(_ context.Context) (string, error) {
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%sinvite/%s", a.baseUrl, a.name), nil)
	req.Header.Set("user_id", fmt.Sprintf("%d", a.secretCode))

	respData, err := a.makeRequest(req)
	if err != nil {
		return "", err
	}

	if respData.Data != nil {
		return respData.Data.(httpserver.UserInviteCodeData).InviteCode, nil
	} else if respData.Err != nil {
		return "", errors.New(*respData.Err)
	} else {
		return "", ErrClient
	}
}

func (a *App) GetUserSecretCode(_ context.Context, password string) error {
	reqBody, _ := json.Marshal(httpserver.GetUserSecretCodeRequest{
		Password: password,
	})
	req, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%sinvite/%s", a.baseUrl, a.name), bytes.NewReader(reqBody))

	respData, err := a.makeRequest(req)
	if err != nil {
		return err
	}

	if respData.Data != nil {
		a.secretCode = respData.Data.(httpserver.UserSecretCodeData).SecretCode
		return nil
	} else if respData.Err != nil {
		return errors.New(*respData.Err)
	} else {
		return ErrClient
	}
}

func (a *App) CreateSession(ctx context.Context, code string, stepsAmount int) (uint64, error) {
	if a.secretCode == 0 {
		return 0, ErrUserNotAuthorized
	}

	ownerCode, err := a.GetUserInviteCode(ctx)
	if err != nil {
		return 0, err
	}

	reqBody, _ := json.Marshal(httpserver.CreateSessionRequest{
		Id01:        ownerCode,
		Id02:        code,
		StepsAmount: stepsAmount,
	})

	req, _ := http.NewRequest(http.MethodPost, a.baseUrl+"session", bytes.NewReader(reqBody))

	respData, err := a.makeRequest(req)
	if err != nil {
		return 0, err
	}

	if respData.Data != nil {
		return respData.Data.(httpserver.CreateSessionData).SessionId, nil
	} else if respData.Err != nil {
		return 0, errors.New(*respData.Err)
	} else {
		return 0, ErrClient
	}
}

func (a *App) GetSession(_ context.Context, sessionId uint64) (int, error) {
	req, _ := http.NewRequest(http.MethodGet, a.baseUrl+"session", nil)
	req.Header.Add("user_id", fmt.Sprintf("%d", a.secretCode))
	q := req.URL.Query()
	q.Add("session_id", fmt.Sprintf("%d", sessionId))
	req.URL.RawQuery = q.Encode()

	respData, err := a.makeRequest(req)
	if err != nil {
		return 0, err
	}

	if respData.Data != nil {
		return respData.Data.(httpserver.GetStepsAmountData).StepsAmount, nil
	} else if respData.Err != nil {
		return 0, errors.New(*respData.Err)
	} else {
		return 0, nil
	}
}

func (a *App) MakeQuestion(_ context.Context, stepIdx int, hiddenQuestion string, openQuestion string) error {
	if a.secretCode == 0 {
		return ErrUserNotAuthorized
	}
	if a.SessionCode == 0 {
		return ErrNoSessionCode
	}

	reqBody, _ := json.Marshal(httpserver.MakeQuestionRequest{
		SessionId:      a.SessionCode,
		StepIdx:        stepIdx,
		HiddenQuestion: hiddenQuestion,
		OpenQuestion:   openQuestion,
	})

	req, _ := http.NewRequest(http.MethodPost, a.baseUrl+"question", bytes.NewReader(reqBody))
	req.Header.Add("user_id", fmt.Sprintf("%d", a.secretCode))

	respData, err := a.makeRequest(req)
	if err != nil {
		return err
	}

	if respData.Err != nil {
		return errors.New(*respData.Err)
	} else {
		return nil
	}
}

func (a *App) MakeAnswer(_ context.Context, stepIdx int, answer string) error {
	if a.secretCode == 0 {
		return ErrUserNotAuthorized
	}
	if a.SessionCode == 0 {
		return ErrNoSessionCode
	}

	reqBody, _ := json.Marshal(httpserver.MakeAnswerRequest{
		SessionId: a.SessionCode,
		StepIdx:   stepIdx,
		Answer:    answer,
	})

	req, _ := http.NewRequest(http.MethodPost, a.baseUrl+"answer", bytes.NewReader(reqBody))
	req.Header.Add("user_id", fmt.Sprintf("%d", a.secretCode))

	respData, err := a.makeRequest(req)
	if err != nil {
		return err
	}

	if respData.Err != nil {
		return errors.New(*respData.Err)
	} else {
		return nil
	}
}

func (a *App) GetQuestion(_ context.Context, stepIdx int) (string, error) {
	if a.secretCode == 0 {
		return "", ErrUserNotAuthorized
	}
	if a.SessionCode == 0 {
		return "", ErrNoSessionCode
	}

	req, _ := http.NewRequest(http.MethodGet, a.baseUrl+"question", nil)
	q := req.URL.Query()
	q.Add("session_id", fmt.Sprintf("%d", a.SessionCode))
	q.Add("step_idx", strconv.Itoa(stepIdx))
	req.URL.RawQuery = q.Encode()
	req.Header.Add("user_id", fmt.Sprintf("%d", a.secretCode))

	respData, err := a.makeRequest(req)
	if err != nil {
		return "", err
	}

	if respData.Data != nil {
		return respData.Data.(httpserver.GetQuestionData).OpenQuestion, nil
	} else if respData.Err != nil {
		return "", errors.New(*respData.Err)
	} else {
		return "", ErrClient
	}
}

func (a *App) GetResults(_ context.Context) (httpserver.GetSessionData, error) {
	if a.secretCode == 0 {
		return httpserver.GetSessionData{}, ErrUserNotAuthorized
	}
	if a.SessionCode == 0 {
		return httpserver.GetSessionData{}, ErrNoSessionCode
	}

	req, _ := http.NewRequest(http.MethodGet, a.baseUrl+"session", nil)
	q := req.URL.Query()
	q.Add("session_id", fmt.Sprintf("%d", a.SessionCode))
	req.URL.RawQuery = q.Encode()
	req.Header.Add("user_id", fmt.Sprintf("%d", a.secretCode))

	respData, err := a.makeRequest(req)
	if err != nil {
		return httpserver.GetSessionData{}, err
	}

	if respData.Data != nil {
		return respData.Data.(httpserver.GetSessionData), nil
	} else if respData.Err != nil {
		return httpserver.GetSessionData{}, errors.New(*respData.Err)
	} else {
		return httpserver.GetSessionData{}, ErrClient
	}
}

func (a *App) makeRequest(req *http.Request) (httpserver.Response, error) {
	resp, err := a.cli.Do(req)
	if err != nil {
		return httpserver.Response{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, _ := io.ReadAll(resp.Body)
	var respData httpserver.Response
	if err := json.Unmarshal(body, &respData); err != nil {
		return httpserver.Response{}, errors.Join(ErrClient, err)
	}
	return respData, nil
}
