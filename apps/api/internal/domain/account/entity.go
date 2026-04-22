package domainaccount

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const RoleUser = "user"

type User struct {
	ID       int64
	Account  string
	Password string
	Nickname string
	Role     string
}

func New(account, password, nickname string) (*User, error) {
	account = strings.TrimSpace(account)
	password = strings.TrimSpace(password)
	nickname = strings.TrimSpace(nickname)

	if account == "" {
		return nil, ErrEmptyAccount
	}
	if password == "" {
		return nil, ErrEmptyPassword
	}
	if nickname == "" {
		return nil, ErrEmptyNickname
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, ErrHashPasswordFailed
	}

	return &User{
		Account:  account,
		Password: string(hashedPassword),
		Nickname: nickname,
		Role:     RoleUser,
	}, nil
}

func RestoreUser(id int64, account, password, nickname, role string) *User {
	account = strings.TrimSpace(account)
	password = strings.TrimSpace(password)
	nickname = strings.TrimSpace(nickname)
	role = strings.TrimSpace(role)
	if role == "" {
		role = RoleUser
	}

	return &User{
		ID:       id,
		Account:  account,
		Password: password,
		Nickname: nickname,
		Role:     role,
	}
}

func (u *User) Authenticate(password string) error {
	password = strings.TrimSpace(password)
	if password == "" {
		return ErrEmptyPassword
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)); err != nil {
		return ErrInvalidCredentials
	}
	return nil
}
