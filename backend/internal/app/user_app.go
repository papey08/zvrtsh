package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"zvrtsh/internal/model"
	userrepo "zvrtsh/internal/repo/user_repo"

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
	users userrepo.UserRepo
}

func newUserApp(users userrepo.UserRepo) *userAppImpl {
	return &userAppImpl{users: users}
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
