package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"zvrtsh/internal/model"
	inviterepo "zvrtsh/internal/repo/invite_repo"
	userrepo "zvrtsh/internal/repo/user_repo"

	"github.com/Pallinder/go-randomdata"
)

const salt = "привет я коза"

type userAppImpl struct {
	users   userrepo.UserRepo
	invites inviterepo.InviteRepo
}

func (u *userAppImpl) RegisterUser(ctx context.Context, name string, password string) (model.User, error) {
	encodedPassword := getHashedString(password)
	secretCode := generateSecretCode()

	return u.users.CreateUser(ctx, name, encodedPassword, secretCode)
}

func generateSecretCode() uint64 {
	return rand.Uint64()
}

func generateInviteCode() string {
	return randomdata.PhoneNumber()
}

func getHashedString(str string) string {
	hash := sha256.New()
	hash.Write([]byte(str + salt))
	hashSum := hash.Sum(nil)
	return hex.EncodeToString(hashSum)
}

func (u *userAppImpl) GetUserInviteCode(ctx context.Context, name string, token uint64) (string, error) {
	user, err := u.users.GetUser(ctx, name)
	if err != nil {
		return "", err
	}

	if user.SecretCode != token {
		return "", model.ErrUserUnauthorized
	}

	inviteCode := generateInviteCode()

	if err := u.invites.CreateInviteCode(ctx, inviteCode, token); err != nil {
		return "", err
	}
	return inviteCode, nil
}

func (u *userAppImpl) GetUserSecretCode(ctx context.Context, name string, password string) (uint64, error) {
	user, err := u.users.GetUser(ctx, name)
	if err != nil {
		return 0, err
	}

	if user.EncodedPassword != getHashedString(password) {
		return 0, model.ErrWrongPassword
	}

	return user.SecretCode, nil
}
