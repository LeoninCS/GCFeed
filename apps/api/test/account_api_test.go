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
	ID             int64  `json:"id"`
	Account        string `json:"account"`
	Nickname       string `json:"nickname"`
	AvatarURL      string `json:"avatar_url"`
	Bio            string `json:"bio"`
	Status         int    `json:"status"`
	Role           string `json:"role"`
	FollowingCount int    `json:"following_count"`
	FollowerCount  int    `json:"follower_count"`
	WorkCount      int    `json:"work_count"`
}

type accountTokenResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresInSeconds int64  `json:"expires_in_seconds"`
}

type publicAccountProfileResponse struct {
	ID             int64  `json:"id"`
	Nickname       string `json:"nickname"`
	AvatarURL      string `json:"avatar_url"`
	Bio            string `json:"bio"`
	FollowingCount int    `json:"following_count"`
	FollowerCount  int    `json:"follower_count"`
	WorkCount      int    `json:"work_count"`
}

// memoryAccountRepo 是账号测试用的内存仓储，模拟真实 Repository 的唯一账号索引。
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

// Save 模拟 account 表插入逻辑，并在账号重复时返回领域错误。
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

// FindByAccount 模拟登录时按账号查询用户。
func (r *memoryAccountRepo) FindByAccount(ctx context.Context, account string) (*domainaccount.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, exists := r.byAccount[account]
	if !exists {
		return nil, domainaccount.ErrUserNotFound
	}
	return cloneUser(r.byID[id]), nil
}

// FindByID 模拟根据 token 中的用户 ID 查询个人资料。
func (r *memoryAccountRepo) FindByID(ctx context.Context, id int64) (*domainaccount.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user, exists := r.byID[id]
	if !exists {
		return nil, domainaccount.ErrUserNotFound
	}
	return cloneUser(user), nil
}

// UpdateProfile 只更新资料字段，与真实仓储保持同样的行为边界。
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

func (r *memoryAccountRepo) SetStatsForTest(userID int64, followingCount int, followerCount int, workCount int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, exists := r.byID[userID]
	if !exists {
		return
	}
	stored.FollowingCount = followingCount
	stored.FollowerCount = followerCount
	stored.WorkCount = workCount
}

// cloneUser 返回副本，避免测试代码直接修改仓储中的内部对象。
func cloneUser(user *domainaccount.User) *domainaccount.User {
	cloned := *user
	return &cloned
}

// TestAccountAPIFlow 覆盖注册、重复注册、登录、读取资料、更新资料和登出完整流程。
func TestAccountAPIFlow(t *testing.T) {
	router := newAccountRouter(t)

	registerResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/users",
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
		"/api/users",
		`{"account":"test","password":"12345678","nickname":"tester"}`,
		"",
	)
	requireStatus(t, duplicateResponse, http.StatusConflict)

	loginResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/sessions",
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

	logoutResponse := performJSONRequest(router, http.MethodDelete, "/api/sessions/current", "", token.AccessToken)
	requireStatus(t, logoutResponse, http.StatusNoContent)
}

// TestAccountAPIValidation 覆盖账号接口的常见参数错误和未登录访问。
func TestAccountAPIValidation(t *testing.T) {
	router := newAccountRouter(t)

	registerResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/users",
		`{"account":"test","password":"","nickname":"tester"}`,
		"",
	)
	requireStatus(t, registerResponse, http.StatusBadRequest)

	loginResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/sessions",
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

// TestPublicAccountProfile 覆盖公开用户主页资料中的关注数、粉丝数和作品数。
func TestPublicAccountProfile(t *testing.T) {
	router, repo := newAccountRouterWithRepo(t)

	registerResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/users",
		`{"account":"creator","password":"12345678","nickname":"creator name"}`,
		"",
	)
	requireStatus(t, registerResponse, http.StatusCreated)

	var created accountProfileResponse
	decodeJSON(t, registerResponse, &created)
	repo.SetStatsForTest(created.ID, 7, 11, 3)

	response := performJSONRequest(router, http.MethodGet, "/api/users/1", "", "")
	requireStatus(t, response, http.StatusOK)

	var profile publicAccountProfileResponse
	decodeJSON(t, response, &profile)
	if profile.ID != created.ID || profile.Nickname != "creator name" || profile.FollowingCount != 7 || profile.FollowerCount != 11 || profile.WorkCount != 3 {
		t.Fatalf("unexpected public profile response: %+v", profile)
	}
}

// registerAndLogin 为需要登录态的测试准备可用 access token。
func registerAndLogin(t *testing.T, router *gin.Engine) string {
	t.Helper()

	registerResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/users",
		`{"account":"login-user","password":"12345678","nickname":"login tester"}`,
		"",
	)
	requireStatus(t, registerResponse, http.StatusCreated)

	loginResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/sessions",
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

// newAccountRouter 只装配账号相关路由，使测试聚焦账号模块。
func newAccountRouter(t *testing.T) *gin.Engine {
	router, _ := newAccountRouterWithRepo(t)
	return router
}

func newAccountRouterWithRepo(t *testing.T) (*gin.Engine, *memoryAccountRepo) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	jwtManager, err := infrajwt.NewManager("test-secret", "15m")
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}
	repo := newMemoryAccountRepo()
	service := applicationaccount.New(repo, jwtManager)
	handler := interfaceshttpaccount.New(service)
	authMiddleware := interfaceshttpmiddleware.NewJWTAuth(jwtManager)

	api := router.Group("/api")
	// 测试路由保持和正式 RESTful 路由一致，便于测试覆盖真实接口路径。
	sessions := api.Group("/sessions")
	sessions.POST("", handler.Login)
	sessions.DELETE("/current", authMiddleware, handler.Logout)

	users := api.Group("/users")
	users.POST("", handler.Register)
	users.GET("/me", authMiddleware, handler.Me)
	users.PATCH("/me", authMiddleware, handler.UpdateMe)
	users.GET("/:userId", handler.Get)

	return router, repo
}

// performJSONRequest 构造 JSON 请求，并在需要时附加 Bearer token。
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

// decodeJSON 解码响应体，失败时输出原始响应内容便于定位问题。
func decodeJSON(t *testing.T, resp *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.Unmarshal(resp.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response body %q: %v", resp.Body.String(), err)
	}
}

// requireStatus 统一断言 HTTP 状态码，失败时把响应体一并打印出来。
func requireStatus(t *testing.T, resp *httptest.ResponseRecorder, expected int) {
	t.Helper()

	if resp.Code != expected {
		t.Fatalf("expected status %d, got %d body=%s", expected, resp.Code, resp.Body.String())
	}
}
