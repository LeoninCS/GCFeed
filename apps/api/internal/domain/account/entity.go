package domainaccount

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const (
	RoleUser     = "user"
	StatusNormal = 1
)

type User struct {
	ID        int64
	Account   string
	Password  string
	Nickname  string
	AvatarURL string
	Bio       string
	Status    int
	Role      string
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
		Status:   StatusNormal,
		Role:     RoleUser,
	}, nil
}

func RestoreUser(id int64, account, password, nickname, avatarURL, bio string, status int, role string) *User {
	account = strings.TrimSpace(account)
	password = strings.TrimSpace(password)
	nickname = strings.TrimSpace(nickname)
	avatarURL = strings.TrimSpace(avatarURL)
	bio = strings.TrimSpace(bio)
	role = strings.TrimSpace(role)
	if status == 0 {
		status = StatusNormal
	}
	if role == "" {
		role = RoleUser
	}

	return &User{
		ID:        id,
		Account:   account,
		Password:  password,
		Nickname:  nickname,
		AvatarURL: avatarURL,
		Bio:       bio,
		Status:    status,
		Role:      role,
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

func (u *User) UpdateProfile(nickname, avatarURL, bio *string) error {
	if nickname == nil && avatarURL == nil && bio == nil {
		return ErrEmptyProfileUpdate
	}

	if nickname != nil {
		value := strings.TrimSpace(*nickname)
		if value == "" {
			return ErrEmptyNickname
		}
		u.Nickname = value
	}
	if avatarURL != nil {
		u.AvatarURL = strings.TrimSpace(*avatarURL)
	}
	if bio != nil {
		u.Bio = strings.TrimSpace(*bio)
	}

	return nil
}
