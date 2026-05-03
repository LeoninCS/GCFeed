package applicationaccount

import (
	domainaccount "GCFeed/internal/domain/account"
	"context"
	"errors"
	"strings"
	"time"
)

var ErrLoadAccountFailed = errors.New("failed to load account")
var ErrSaveAccountFailed = errors.New("failed to save account")
var ErrUpdateAccountFailed = errors.New("failed to update account")
var ErrSignAccessTokenFailed = errors.New("failed to sign access token")

type TokenSigner interface {
	SignAccessToken(userID int64, role string) (string, error)
	AccessTTL() time.Duration
}

type Service struct {
	repo   domainaccount.Repository
	signer TokenSigner
}

type LoginResult struct {
	AccessToken      string
	TokenType        string
	ExpiresInSeconds int64
}

type Profile struct {
	ID        int64
	Account   string
	Nickname  string
	AvatarURL string
	Bio       string
	Status    int
	Role      string
}

func NewService(repo domainaccount.Repository, signer TokenSigner) *Service {
	return &Service{
		repo:   repo,
		signer: signer,
	}
}

func (s *Service) Register(ctx context.Context, account, password, nickname string) (*Profile, error) {
	user, err := domainaccount.New(account, password, nickname)
	if err != nil {
		return nil, err
	}

	err = s.repo.Save(ctx, user)
	if err != nil {
		if errors.Is(err, domainaccount.ErrAccountAlreadyExists) {
			return nil, domainaccount.ErrAccountAlreadyExists
		}
		return nil, ErrSaveAccountFailed
	}
	return profileFromUser(user), nil
}

func (s *Service) Login(ctx context.Context, account, password string) (*LoginResult, error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return nil, domainaccount.ErrEmptyAccount
	}

	user, err := s.repo.FindByAccount(ctx, account)
	if err != nil {
		if errors.Is(err, domainaccount.ErrUserNotFound) {
			return nil, domainaccount.ErrInvalidCredentials
		}
		return nil, ErrLoadAccountFailed
	}
	if err := user.Authenticate(password); err != nil {
		return nil, err
	}

	accessToken, err := s.signer.SignAccessToken(user.ID, user.Role)
	if err != nil {
		return nil, ErrSignAccessTokenFailed
	}

	return &LoginResult{
		AccessToken:      accessToken,
		TokenType:        "Bearer",
		ExpiresInSeconds: int64(s.signer.AccessTTL().Seconds()),
	}, nil
}

func (s *Service) GetProfile(ctx context.Context, userID int64) (*Profile, error) {
	if userID <= 0 {
		return nil, domainaccount.ErrInvalidUserID
	}

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domainaccount.ErrUserNotFound) {
			return nil, domainaccount.ErrUserNotFound
		}
		return nil, ErrLoadAccountFailed
	}

	return profileFromUser(user), nil
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, nickname, avatarURL, bio *string) (*Profile, error) {
	if userID <= 0 {
		return nil, domainaccount.ErrInvalidUserID
	}

	user, err := s.repo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, domainaccount.ErrUserNotFound) {
			return nil, domainaccount.ErrUserNotFound
		}
		return nil, ErrLoadAccountFailed
	}
	if err := user.UpdateProfile(nickname, avatarURL, bio); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateProfile(ctx, user); err != nil {
		if errors.Is(err, domainaccount.ErrUserNotFound) {
			return nil, domainaccount.ErrUserNotFound
		}
		return nil, ErrUpdateAccountFailed
	}

	return profileFromUser(user), nil
}

func profileFromUser(user *domainaccount.User) *Profile {
	return &Profile{
		ID:        user.ID,
		Account:   user.Account,
		Nickname:  user.Nickname,
		AvatarURL: user.AvatarURL,
		Bio:       user.Bio,
		Status:    user.Status,
		Role:      user.Role,
	}
}
