package test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	applicationaccount "GCFeed/internal/application/account"
	domainaccount "GCFeed/internal/domain/account"
	infrajwt "GCFeed/internal/infra/jwt"
	interfaceshttpaccount "GCFeed/internal/interfaces/http/account"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type accountProfileResponse struct {
	ID        int64  `json:"id"`
	Account   string `json:"account"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatar_url"`
	Bio       string `json:"bio"`
	Status    int    `json:"status"`
	Role      string `json:"role"`
}

type accountTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

type memoryAccountRepo struct {
	mu        sync.Mutex
	nextID    int64
	byID      map[int64]*domainaccount.User
	byAccount map[string]int64
}

func newMemoryAccountRepo() *memoryAccountRepo {
	return &memoryAccountRepo{
		nextID:    1,
		byID:      map[int64]*domainaccount.User{},
		byAccount: map[string]int64{},
	}
}

func (r *memoryAccountRepo) Save(ctx context.Context, user *domainaccount.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.byAccount[user.Account]; exists {
		return domainaccount.ErrAccountAlreadyExists
	}

	user.ID = r.nextID
	r.nextID++
	r.byID[user.ID] = cloneUser(user)
	r.byAccount[user.Account] = user.ID
	return nil
}

func (r *memoryAccountRepo) FindByAccount(ctx context.Context, account string) (*domainaccount.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, exists := r.byAccount[account]
	if !exists {
		return nil, domainaccount.ErrUserNotFound
	}
	return cloneUser(r.byID[id]), nil
}

func (r *memoryAccountRepo) FindByID(ctx context.Context, id int64) (*domainaccount.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.byID[id]
	if !exists {
		return nil, domainaccount.ErrUserNotFound
	}
	return cloneUser(user), nil
}

func (r *memoryAccountRepo) UpdateProfile(ctx context.Context, user *domainaccount.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, exists := r.byID[user.ID]
	if !exists {
		return domainaccount.ErrUserNotFound
	}
	stored.Nickname = user.Nickname
	stored.AvatarURL = user.AvatarURL
	stored.Bio = user.Bio
	return nil
}

func cloneUser(user *domainaccount.User) *domainaccount.User {
	cloned := *user
	return &cloned
}

func TestAccountAPIFlow(t *testing.T) {
	router := newAccountRouter(t)

	registerResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/auth/register",
		`{"account":"test","password":"12345678","nickname":"tester"}`,
		"",
	)
	requireStatus(t, registerResponse, http.StatusCreated)

	var created accountProfileResponse
	decodeJSON(t, registerResponse, &created)
	if created.ID == 0 {
		t.Fatalf("expected created user id")
	}
	if created.Account != "test" || created.Nickname != "tester" || created.Status != domainaccount.StatusNormal || created.Role != domainaccount.RoleUser {
		t.Fatalf("unexpected register response: %+v", created)
	}

	duplicateResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/auth/register",
		`{"account":"test","password":"12345678","nickname":"tester"}`,
		"",
	)
	requireStatus(t, duplicateResponse, http.StatusConflict)

	loginResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/auth/login/password",
		`{"account":"test","password":"12345678"}`,
		"",
	)
	requireStatus(t, loginResponse, http.StatusOK)

	var token accountTokenResponse
	decodeJSON(t, loginResponse, &token)
	if token.AccessToken == "" || token.TokenType != "Bearer" || token.ExpiresInSeconds != 900 {
		t.Fatalf("unexpected login response: %+v", token)
	}

	meResponse := performJSONRequest(router, http.MethodGet, "/api/users/me", "", token.AccessToken)
	requireStatus(t, meResponse, http.StatusOK)

	var profile accountProfileResponse
	decodeJSON(t, meResponse, &profile)
	if profile.ID != created.ID || profile.Account != "test" || profile.Nickname != "tester" {
		t.Fatalf("unexpected profile response: %+v", profile)
	}

	updateResponse := performJSONRequest(
		router,
		http.MethodPatch,
		"/api/users/me",
		`{"nickname":"tester-updated","avatar_url":"https://example.com/avatar.png","bio":"hello feed"}`,
		token.AccessToken,
	)
	requireStatus(t, updateResponse, http.StatusOK)

	var updated accountProfileResponse
	decodeJSON(t, updateResponse, &updated)
	if updated.Nickname != "tester-updated" || updated.AvatarURL != "https://example.com/avatar.png" || updated.Bio != "hello feed" {
		t.Fatalf("unexpected updated profile response: %+v", updated)
	}

	logoutResponse := performJSONRequest(router, http.MethodPost, "/api/auth/logout", "", token.AccessToken)
	requireStatus(t, logoutResponse, http.StatusNoContent)
}

func TestAccountAPIValidation(t *testing.T) {
	router := newAccountRouter(t)

	registerResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/auth/register",
		`{"account":"test","password":"","nickname":"tester"}`,
		"",
	)
	requireStatus(t, registerResponse, http.StatusBadRequest)

	loginResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/auth/login/password",
		`{"account":"","password":"12345678"}`,
		"",
	)
	requireStatus(t, loginResponse, http.StatusBadRequest)

	unauthorizedMeResponse := performJSONRequest(router, http.MethodGet, "/api/users/me", "", "")
	requireStatus(t, unauthorizedMeResponse, http.StatusUnauthorized)

	token := registerAndLogin(t, router)

	emptyPatchResponse := performJSONRequest(router, http.MethodPatch, "/api/users/me", `{}`, token)
	requireStatus(t, emptyPatchResponse, http.StatusBadRequest)

	emptyNicknameResponse := performJSONRequest(
		router,
		http.MethodPatch,
		"/api/users/me",
		`{"nickname":"   "}`,
		token,
	)
	requireStatus(t, emptyNicknameResponse, http.StatusBadRequest)
}

func registerAndLogin(t *testing.T, router *gin.Engine) string {
	t.Helper()

	registerResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/auth/register",
		`{"account":"login-user","password":"12345678","nickname":"login tester"}`,
		"",
	)
	requireStatus(t, registerResponse, http.StatusCreated)

	loginResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/auth/login/password",
		`{"account":"login-user","password":"12345678"}`,
		"",
	)
	requireStatus(t, loginResponse, http.StatusOK)

	var token accountTokenResponse
	decodeJSON(t, loginResponse, &token)
	if token.AccessToken == "" {
		t.Fatalf("expected access token")
	}
	return token.AccessToken
}

func newAccountRouter(t *testing.T) *gin.Engine {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	jwtManager, err := infrajwt.NewManager("test-secret", "15m")
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}
	repo := newMemoryAccountRepo()
	service := applicationaccount.NewService(repo, jwtManager)
	handler := interfaceshttpaccount.NewHandler(service)
	authMiddleware := interfaceshttpmiddleware.NewJWTAuth(jwtManager)

	api := router.Group("/api")
	auth := api.Group("/auth")
	auth.POST("/register", handler.Register)
	auth.POST("/login/password", handler.Login)
	auth.POST("/logout", authMiddleware, handler.Logout)

	users := api.Group("/users", authMiddleware)
	users.GET("/me", handler.Me)
	users.PATCH("/me", handler.UpdateMe)

	return router
}

func performJSONRequest(router *gin.Engine, method, path, body, accessToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func decodeJSON(t *testing.T, resp *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.Unmarshal(resp.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response body %q: %v", resp.Body.String(), err)
	}
}

func requireStatus(t *testing.T, resp *httptest.ResponseRecorder, expected int) {
	t.Helper()

	if resp.Code != expected {
		t.Fatalf("expected status %d, got %d body=%s", expected, resp.Code, resp.Body.String())
	}
}
