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

// TokenSigner 是应用层依赖的最小 JWT 能力，账号服务只关心“签发 token”和“过期时间”。
type TokenSigner interface {
	SignAccessToken(userID int64, role string) (string, error)
	AccessTTL() time.Duration
}

// Service 编排账号用例：注册、登录、读取资料、更新资料。
type Service struct {
	repo   domainaccount.Repository
	signer TokenSigner
}

// LoginResult 是登录成功后返回给 HTTP 层的 token 数据。
type LoginResult struct {
	AccessToken      string
	TokenType        string
	ExpiresInSeconds int64
}

// Profile 是应用层对外暴露的用户资料视图，屏蔽密码等敏感字段。
type Profile struct {
	ID             int64
	Account        string
	Nickname       string
	AvatarURL      string
	Bio            string
	Status         int
	Role           string
	FollowingCount int
	FollowerCount  int
	WorkCount      int
}

func New(repo domainaccount.Repository, signer TokenSigner) *Service {
	return &Service{
		repo:   repo,
		signer: signer,
	}
}

// Register 创建新用户：领域层负责校验和加密密码，仓储层负责持久化。
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

// Login 完成账号密码登录，认证通过后签发访问 token。
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

	// token 内写入用户 ID 和角色，后续鉴权中间件会解析并放入请求上下文。
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

// GetProfile 根据登录态中的用户 ID 读取当前用户资料。
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

// GetPublicProfile 根据用户 ID 读取公开资料，用于访问他人主页。
func (s *Service) GetPublicProfile(ctx context.Context, userID int64) (*Profile, error) {
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

// UpdateProfile 支持部分更新，nil 表示该字段没有出现在请求体中。
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

// profileFromUser 把领域用户转换成安全的资料对象，避免向外暴露密码哈希。
func profileFromUser(user *domainaccount.User) *Profile {
	return &Profile{
		ID:             user.ID,
		Account:        user.Account,
		Nickname:       user.Nickname,
		AvatarURL:      user.AvatarURL,
		Bio:            user.Bio,
		Status:         user.Status,
		Role:           user.Role,
		FollowingCount: user.FollowingCount,
		FollowerCount:  user.FollowerCount,
		WorkCount:      user.WorkCount,
	}
}
