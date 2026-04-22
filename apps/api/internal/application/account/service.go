package applicationaccount

import (
	"context"
	"errors"
	domainaccount "feedsystem_video_hard/internal/domain/account"
	"strings"
	"time"
)

var ErrLoadAccountFailed = errors.New("failed to load account")
var ErrSaveAccountFailed = errors.New("failed to save account")
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
	ID       int64
	Account  string
	Nickname string
	Role     string
}

func NewService(repo domainaccount.Repository, signer TokenSigner) *Service {
	return &Service{
		repo:   repo,
		signer: signer,
	}
}

func (s *Service) Register(ctx context.Context, account, password, nickname string) error {
	user, err := domainaccount.New(account, password, nickname)
	if err != nil {
		return err
	}

	err = s.repo.Save(ctx, user)
	if err != nil {
		if errors.Is(err, domainaccount.ErrAccountAlreadyExists) {
			return domainaccount.ErrAccountAlreadyExists
		}
		return ErrSaveAccountFailed
	}
	return nil
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
