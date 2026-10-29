package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
	"zvrtsh/internal/model"
	inviterepo "zvrtsh/internal/repo/invite_repo"
	userrepo "zvrtsh/internal/repo/user_repo"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	minNameLen     = 4
	maxNameLen     = 30
	minPasswordLen = 4
	maxPasswordLen = 30

	passwordSymbols = "!@#$%^&*()-_=+[]{}\\|/'\";:,.<>?"
)

// dummyHash is compared with the given password when the user does not exist,
// so that the response time does not reveal whether a name is registered.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy password"), bcrypt.DefaultCost)

type userAppImpl struct {
	users     userrepo.UserRepo
	invites   inviterepo.InviteRepo
	inviteTTL time.Duration
	now       func() time.Time
}

func newUserApp(users userrepo.UserRepo, invites inviterepo.InviteRepo, inviteTTL time.Duration, now func() time.Time) *userAppImpl {
	return &userAppImpl{
		users:     users,
		invites:   invites,
		inviteTTL: inviteTTL,
		now:       now,
	}
}

func (u *userAppImpl) RegisterUser(ctx context.Context, name string, password string) (model.User, error) {
	if !isValidName(name) || !isValidPassword(password) {
		return model.User{}, model.ErrValidation
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, err
	}
	token, err := generateToken()
	if err != nil {
		return model.User{}, err
	}

	user := model.User{
		Name:            name,
		EncodedPassword: string(hash),
		Token:           token,
	}
	if err = u.users.CreateUser(ctx, user); err != nil {
		return model.User{}, err
	}
	return user, nil
}

func (u *userAppImpl) CreateToken(ctx context.Context, name string, password string) (string, error) {
	user, err := u.users.GetUser(ctx, name)
	if errors.Is(err, model.ErrUserNotFound) {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return "", model.ErrUnauthorized
	} else if err != nil {
		return "", err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.EncodedPassword), []byte(password)) != nil {
		return "", model.ErrUnauthorized
	}
	return user.Token, nil
}

func (u *userAppImpl) Authenticate(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", model.ErrUnauthorized
	}

	user, err := u.users.GetUserByToken(ctx, token)
	if errors.Is(err, model.ErrUserNotFound) {
		return "", model.ErrUnauthorized
	} else if err != nil {
		return "", err
	}
	return user.Name, nil
}

func (u *userAppImpl) CreateInvite(ctx context.Context, userName string, stepsAmount int) (model.Invite, error) {
	if stepsAmount < 1 || stepsAmount > maxStepsAmount {
		return model.Invite{}, fmt.Errorf("%w: steps amount must be between 1 and %d", model.ErrValidation, maxStepsAmount)
	}

	now := u.now()
	invite := model.Invite{
		Code:         uuid.New().String(),
		UserName:     userName,
		StepsAmount:  stepsAmount,
		CreationDate: now,
		ExpiresAt:    now.Add(u.inviteTTL),
	}
	if err := u.invites.CreateInvite(ctx, invite); err != nil {
		return model.Invite{}, err
	}
	return invite, nil
}

// DeleteInvite cancels the active invite of the user, if there is one.
func (u *userAppImpl) DeleteInvite(ctx context.Context, userName string) error {
	return u.invites.DeleteInvite(ctx, userName)
}

func (u *userAppImpl) purgeExpiredInvites(ctx context.Context) error {
	return u.invites.DeleteExpiredInvites(ctx, u.now())
}

// isValidName checks that the name consists of latin letters, digits and _.
func isValidName(name string) bool {
	if len(name) < minNameLen || len(name) > maxNameLen {
		return false
	}
	for _, r := range name {
		if !isASCIILetter(r) && !isASCIIDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

// isValidPassword checks that the password consists of latin letters, digits and punctuation.
func isValidPassword(password string) bool {
	if len(password) < minPasswordLen || len(password) > maxPasswordLen {
		return false
	}
	for _, r := range password {
		if !isASCIILetter(r) && !isASCIIDigit(r) && !strings.ContainsRune(passwordSymbols, r) {
			return false
		}
	}
	return true
}

func isASCIILetter(r rune) bool {
	return 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z'
}

func isASCIIDigit(r rune) bool {
	return '0' <= r && r <= '9'
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
