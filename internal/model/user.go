package model

type User struct {
	Name            string
	EncodedPassword string
	SecretCode      uint64
}
