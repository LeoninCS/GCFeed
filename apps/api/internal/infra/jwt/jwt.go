package infrajwt

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	jwtlib "github.com/golang-jwt/jwt/v5"
)

const (
	defaultAccessTTL = 15 * time.Minute
	TokenTypeAccess  = "access"
)

var ErrEmptyJWTSecret = errors.New("jwt secret is required")
var ErrParseAccessTTL = errors.New("parse jwt access_ttl failed")
var ErrEmptyToken = errors.New("token is empty")
var ErrParseJWTToken = errors.New("parse jwt token failed")
var ErrInvalidTokenType = errors.New("token type invalid")
var ErrInvalidTokenUserID = errors.New("token user id invalid")
var ErrInvalidUserID = errors.New("user id must be positive")
var ErrGenerateTokenJTI = errors.New("generate token jti failed")
var ErrSignJWTToken = errors.New("sign jwt token failed")
var ErrInvalidTTL = errors.New("ttl must be positive")

type Claims struct {
	UserID    int64  `json:"uid"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	JWTID     string `json:"jti"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

type Manager struct {
	secret    []byte
	accessTTL time.Duration
}

type tokenClaims struct {
	UserID    int64  `json:"uid"`
	Role      string `json:"role"`
	TokenType string `json:"token_type"`
	jwtlib.RegisteredClaims
}

func NewManager(secret, accessTTL string) (*Manager, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, ErrEmptyJWTSecret
	}

	accessDuration, err := parseTTL(accessTTL, defaultAccessTTL)
	if err != nil {
		return nil, ErrParseAccessTTL
	}

	return &Manager{
		secret:    []byte(secret),
		accessTTL: accessDuration,
	}, nil
}

func (m *Manager) AccessTTL() time.Duration {
	return m.accessTTL
}

func (m *Manager) SignAccessToken(userID int64, role string) (string, error) {
	return m.signToken(userID, role, TokenTypeAccess, m.accessTTL)
}

func (m *Manager) ParseAndValidateToken(token, expectedType string) (*Claims, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, ErrEmptyToken
	}

	parsedClaims := &tokenClaims{}
	_, err := jwtlib.ParseWithClaims(
		token,
		parsedClaims,
		func(token *jwtlib.Token) (any, error) {
			return m.secret, nil
		},
		jwtlib.WithValidMethods([]string{jwtlib.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, ErrParseJWTToken
	}

	if parsedClaims.TokenType != expectedType {
		return nil, ErrInvalidTokenType
	}
	if parsedClaims.UserID <= 0 {
		return nil, ErrInvalidTokenUserID
	}

	return &Claims{
		UserID:    parsedClaims.UserID,
		Role:      parsedClaims.Role,
		TokenType: parsedClaims.TokenType,
		JWTID:     parsedClaims.ID,
		IssuedAt:  claimTimeUnix(parsedClaims.IssuedAt),
		ExpiresAt: claimTimeUnix(parsedClaims.ExpiresAt),
	}, nil
}

func (m *Manager) signToken(userID int64, role, tokenType string, ttl time.Duration) (string, error) {
	if userID <= 0 {
		return "", ErrInvalidUserID
	}

	role = strings.TrimSpace(role)
	if role == "" {
		role = "user"
	}

	now := time.Now()
	jti, err := randomID(16)
	if err != nil {
		return "", ErrGenerateTokenJTI
	}
	claims := tokenClaims{
		UserID:    userID,
		Role:      role,
		TokenType: tokenType,
		RegisteredClaims: jwtlib.RegisteredClaims{
			IssuedAt:  jwtlib.NewNumericDate(now),
			ExpiresAt: jwtlib.NewNumericDate(now.Add(ttl)),
			ID:        jti,
		},
	}

	token := jwtlib.NewWithClaims(jwtlib.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(m.secret)
	if err != nil {
		return "", ErrSignJWTToken
	}

	return signedToken, nil
}

func parseTTL(raw string, fallback time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	ttl, err := time.ParseDuration(raw)
	if err != nil {
		return 0, err
	}
	if ttl <= 0 {
		return 0, ErrInvalidTTL
	}
	return ttl, nil
}

func randomID(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func claimTimeUnix(value *jwtlib.NumericDate) int64 {
	if value == nil {
		return 0
	}
	return value.Unix()
}
