package test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	applicationvideo "GCFeed/internal/application/video"
	domainvideo "GCFeed/internal/domain/video"
	infrajwt "GCFeed/internal/infra/jwt"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	interfaceshttpvideo "GCFeed/internal/interfaces/http/video"

	"github.com/gin-gonic/gin"
)

type videoAPIResponse struct {
	ID            int64      `json:"id"`
	AuthorID      int64      `json:"author_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	MediaURL      string     `json:"media_url"`
	CoverURL      string     `json:"cover_url"`
	Status        int        `json:"status"`
	LikeCount     int        `json:"like_count"`
	CommentCount  int        `json:"comment_count"`
	FavoriteCount int        `json:"favorite_count"`
	PublishedAt   *time.Time `json:"published_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type videoListAPIResponse struct {
	Items  []videoAPIResponse `json:"items"`
	Limit  int                `json:"limit"`
	Offset int                `json:"offset"`
}

// memoryVideoRepo 是视频测试用的内存仓储，模拟发布、查询、幂等和软删除。
type memoryVideoRepo struct {
	mu            sync.Mutex
	nextID        int64
	byID          map[int64]*domainvideo.Video
	byIdempotency map[string]int64
}

type memoryVideoPublisher struct {
	mu     sync.Mutex
	events []*applicationvideo.PublishedEvent
}

func newMemoryVideoRepo() *memoryVideoRepo {
	return &memoryVideoRepo{
		nextID:        1,
		byID:          map[int64]*domainvideo.Video{},
		byIdempotency: map[string]int64{},
	}
}

// Save 模拟视频创建，并维护作者+幂等键的唯一约束。
func (r *memoryVideoRepo) Save(ctx context.Context, video *domainvideo.Video) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if video.IdempotencyKey != "" {
		key := memoryVideoIdempotencyKey(video.AuthorID, video.IdempotencyKey)
		if _, exists := r.byIdempotency[key]; exists {
			// 与真实数据库唯一键冲突保持同样错误，方便测试应用层重试逻辑。
			return domainvideo.ErrDuplicateIdempotencyKey
		}
	}

	now := time.Now()
	video.ID = r.nextID
	r.nextID++
	if video.CreatedAt.IsZero() {
		video.CreatedAt = now
	}
	video.UpdatedAt = now
	r.byID[video.ID] = cloneVideo(video)
	if video.IdempotencyKey != "" {
		r.byIdempotency[memoryVideoIdempotencyKey(video.AuthorID, video.IdempotencyKey)] = video.ID
	}
	return nil
}

// FindByID 只返回已发布视频，模拟公开详情接口的可见性规则。
func (r *memoryVideoRepo) FindByID(ctx context.Context, id int64) (*domainvideo.Video, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	video, exists := r.byID[id]
	if !exists || video.Status != domainvideo.StatusPublished {
		return nil, domainvideo.ErrVideoNotFound
	}
	return cloneVideo(video), nil
}

// FindByIDAnyStatus 返回任意状态视频，删除流程需要读取已删除状态。
func (r *memoryVideoRepo) FindByIDAnyStatus(ctx context.Context, id int64) (*domainvideo.Video, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	video, exists := r.byID[id]
	if !exists {
		return nil, domainvideo.ErrVideoNotFound
	}
	return cloneVideo(video), nil
}

// FindByAuthorAndIdempotencyKey 模拟发布接口通过幂等键找回已有视频。
func (r *memoryVideoRepo) FindByAuthorAndIdempotencyKey(ctx context.Context, authorID int64, key string) (*domainvideo.Video, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, exists := r.byIdempotency[memoryVideoIdempotencyKey(authorID, key)]
	if !exists {
		return nil, domainvideo.ErrVideoNotFound
	}
	return cloneVideo(r.byID[id]), nil
}

// ListByAuthor 模拟作者作品列表，按发布时间和 ID 倒序排序。
func (r *memoryVideoRepo) ListByAuthor(ctx context.Context, authorID int64, limit, offset int) ([]*domainvideo.Video, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	videos := make([]*domainvideo.Video, 0)
	for _, video := range r.byID {
		if video.AuthorID == authorID && video.Status == domainvideo.StatusPublished {
			videos = append(videos, cloneVideo(video))
		}
	}
	sort.Slice(videos, func(i, j int) bool {
		left := publishedAtUnix(videos[i])
		right := publishedAtUnix(videos[j])
		if left == right {
			return videos[i].ID > videos[j].ID
		}
		return left > right
	})

	if offset >= len(videos) {
		return []*domainvideo.Video{}, nil
	}
	end := offset + limit
	if end > len(videos) {
		end = len(videos)
	}
	return videos[offset:end], nil
}

// UpdateStatus 模拟视频状态更新，当前用于软删除。
func (r *memoryVideoRepo) UpdateStatus(ctx context.Context, video *domainvideo.Video) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	stored, exists := r.byID[video.ID]
	if !exists {
		return domainvideo.ErrVideoNotFound
	}
	stored.Status = video.Status
	stored.UpdatedAt = time.Now()
	return nil
}

func (p *memoryVideoPublisher) PublishVideoPublished(ctx context.Context, event *applicationvideo.PublishedEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events = append(p.events, event)
	return nil
}

func (p *memoryVideoPublisher) EventCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.events)
}

// TestVideoAPIFlow 覆盖视频发布、幂等重放、详情、列表、删除和重复删除。
func TestVideoAPIFlow(t *testing.T) {
	router, jwtManager := newVideoRouter(t)
	token := signTestToken(t, jwtManager, 42)

	createResponse := performVideoJSONRequest(
		router,
		http.MethodPost,
		"/api/videos",
		`{"title":"first video","description":"hello timeline","media_url":"https://example.com/video.mp4","cover_url":"https://example.com/cover.jpg"}`,
		token,
		"create-video-1",
	)
	requireStatus(t, createResponse, http.StatusCreated)

	var created videoAPIResponse
	decodeJSON(t, createResponse, &created)
	if created.ID == 0 || created.AuthorID != 42 || created.Status != domainvideo.StatusPublished {
		t.Fatalf("unexpected create response: %+v", created)
	}
	if created.Title != "first video" || created.Description != "hello timeline" || created.MediaURL == "" || created.CoverURL == "" || created.PublishedAt == nil {
		t.Fatalf("unexpected create response: %+v", created)
	}

	replayResponse := performVideoJSONRequest(
		router,
		http.MethodPost,
		"/api/videos",
		`{"title":"changed title","description":"changed description","media_url":"https://example.com/changed.mp4","cover_url":"https://example.com/changed.jpg"}`,
		token,
		"create-video-1",
	)
	requireStatus(t, replayResponse, http.StatusOK)

	var replayed videoAPIResponse
	decodeJSON(t, replayResponse, &replayed)
	if replayed.ID != created.ID || replayed.Title != created.Title {
		t.Fatalf("unexpected replay response: %+v", replayed)
	}

	getResponse := performJSONRequest(router, http.MethodGet, fmt.Sprintf("/api/videos/%d", created.ID), "", "")
	requireStatus(t, getResponse, http.StatusOK)

	listResponse := performJSONRequest(router, http.MethodGet, "/api/users/42/videos?limit=10&offset=0", "", "")
	requireStatus(t, listResponse, http.StatusOK)

	var list videoListAPIResponse
	decodeJSON(t, listResponse, &list)
	if len(list.Items) != 1 || list.Items[0].ID != created.ID || list.Limit != 10 || list.Offset != 0 {
		t.Fatalf("unexpected author list response: %+v", list)
	}

	mineResponse := performJSONRequest(router, http.MethodGet, "/api/users/me/videos", "", token)
	requireStatus(t, mineResponse, http.StatusOK)

	var mine videoListAPIResponse
	decodeJSON(t, mineResponse, &mine)
	if len(mine.Items) != 1 || mine.Items[0].ID != created.ID {
		t.Fatalf("unexpected mine list response: %+v", mine)
	}

	deleteResponse := performJSONRequest(router, http.MethodDelete, fmt.Sprintf("/api/videos/%d", created.ID), "", token)
	requireStatus(t, deleteResponse, http.StatusNoContent)

	getDeletedResponse := performJSONRequest(router, http.MethodGet, fmt.Sprintf("/api/videos/%d", created.ID), "", "")
	requireStatus(t, getDeletedResponse, http.StatusNotFound)

	repeatDeleteResponse := performJSONRequest(router, http.MethodDelete, fmt.Sprintf("/api/videos/%d", created.ID), "", token)
	requireStatus(t, repeatDeleteResponse, http.StatusNoContent)

	emptyListResponse := performJSONRequest(router, http.MethodGet, "/api/users/42/videos", "", "")
	requireStatus(t, emptyListResponse, http.StatusOK)

	var emptyList videoListAPIResponse
	decodeJSON(t, emptyListResponse, &emptyList)
	if len(emptyList.Items) != 0 {
		t.Fatalf("unexpected empty list response: %+v", emptyList)
	}
}

func TestVideoPublishedEvent(t *testing.T) {
	publisher := &memoryVideoPublisher{}
	router, jwtManager, _ := newVideoRouterWithPublisher(t, publisher)
	token := signTestToken(t, jwtManager, 42)

	createResponse := performVideoJSONRequest(
		router,
		http.MethodPost,
		"/api/videos",
		`{"title":"event video","description":"event description","media_url":"https://example.com/video.mp4","cover_url":"https://example.com/cover.jpg"}`,
		token,
		"create-video-event",
	)
	requireStatus(t, createResponse, http.StatusCreated)
	if publisher.EventCount() != 1 {
		t.Fatalf("unexpected published event count after create: %d", publisher.EventCount())
	}

	replayResponse := performVideoJSONRequest(
		router,
		http.MethodPost,
		"/api/videos",
		`{"title":"changed","description":"changed","media_url":"https://example.com/changed.mp4","cover_url":"https://example.com/changed.jpg"}`,
		token,
		"create-video-event",
	)
	requireStatus(t, replayResponse, http.StatusOK)
	if publisher.EventCount() != 1 {
		t.Fatalf("unexpected published event count after replay: %d", publisher.EventCount())
	}
}

// TestVideoAPIValidation 覆盖未登录、参数错误、资源缺失和权限错误。
func TestVideoAPIValidation(t *testing.T) {
	router, jwtManager := newVideoRouter(t)
	token := signTestToken(t, jwtManager, 42)

	unauthorizedCreateResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/videos",
		`{"title":"first video","description":"hello timeline","media_url":"https://example.com/video.mp4","cover_url":"https://example.com/cover.jpg"}`,
		"",
	)
	requireStatus(t, unauthorizedCreateResponse, http.StatusUnauthorized)

	emptyTitleResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/videos",
		`{"title":"   ","media_url":"https://example.com/video.mp4","cover_url":"https://example.com/cover.jpg"}`,
		token,
	)
	requireStatus(t, emptyTitleResponse, http.StatusBadRequest)

	badIDResponse := performJSONRequest(router, http.MethodGet, "/api/videos/abc", "", "")
	requireStatus(t, badIDResponse, http.StatusBadRequest)

	missingResponse := performJSONRequest(router, http.MethodGet, "/api/videos/404", "", "")
	requireStatus(t, missingResponse, http.StatusNotFound)

	createResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/videos",
		`{"title":"owned video","description":"owned description","media_url":"https://example.com/video.mp4","cover_url":"https://example.com/cover.jpg"}`,
		token,
	)
	requireStatus(t, createResponse, http.StatusCreated)

	var created videoAPIResponse
	decodeJSON(t, createResponse, &created)

	otherToken := signTestToken(t, jwtManager, 77)
	forbiddenDeleteResponse := performJSONRequest(router, http.MethodDelete, fmt.Sprintf("/api/videos/%d", created.ID), "", otherToken)
	requireStatus(t, forbiddenDeleteResponse, http.StatusForbidden)

	badUserListResponse := performJSONRequest(router, http.MethodGet, "/api/users/abc/videos", "", "")
	requireStatus(t, badUserListResponse, http.StatusBadRequest)

	badPaginationResponse := performJSONRequest(router, http.MethodGet, "/api/users/42/videos?limit=0", "", "")
	requireStatus(t, badPaginationResponse, http.StatusBadRequest)
}

// newVideoRouter 只装配视频相关路由，便于视频模块独立测试。
func newVideoRouter(t *testing.T) (*gin.Engine, *infrajwt.Manager) {
	router, jwtManager, _ := newVideoRouterWithPublisher(t, nil)
	return router, jwtManager
}

func newVideoRouterWithPublisher(t *testing.T, publisher applicationvideo.PublishedEventPublisher) (*gin.Engine, *infrajwt.Manager, *memoryVideoRepo) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	jwtManager, err := infrajwt.NewManager("test-secret", "15m")
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}

	repo := newMemoryVideoRepo()
	options := []applicationvideo.Option{}
	if publisher != nil {
		options = append(options, applicationvideo.WithPublishedEventPublisher(publisher))
	}
	service := applicationvideo.New(repo, options...)
	handler := interfaceshttpvideo.New(service)
	authMiddleware := interfaceshttpmiddleware.NewJWTAuth(jwtManager)

	api := router.Group("/api")
	videos := api.Group("/videos")
	videos.POST("", authMiddleware, handler.Create)
	videos.GET("/:videoId", handler.Get)
	videos.DELETE("/:videoId", authMiddleware, handler.Delete)
	users := api.Group("/users")
	users.GET("/me/videos", authMiddleware, handler.ListMine)
	users.GET("/:userId/videos", handler.ListByAuthor)

	return router, jwtManager, repo
}

// signTestToken 生成测试用 access token，让接口走真实 JWT 中间件。
func signTestToken(t *testing.T, jwtManager *infrajwt.Manager, userID int64) string {
	t.Helper()

	token, err := jwtManager.SignAccessToken(userID, "user")
	if err != nil {
		t.Fatalf("sign test token: %v", err)
	}
	return token
}

// performVideoJSONRequest 构造视频接口请求，并支持附加幂等键。
func performVideoJSONRequest(router *gin.Engine, method, path, body, accessToken, idempotencyKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

// cloneVideo 返回视频副本，并复制发布时间指针指向的值。
func cloneVideo(video *domainvideo.Video) *domainvideo.Video {
	cloned := *video
	if video.PublishedAt != nil {
		publishedAt := *video.PublishedAt
		cloned.PublishedAt = &publishedAt
	}
	return &cloned
}

// memoryVideoIdempotencyKey 模拟数据库里的作者+幂等键唯一索引。
func memoryVideoIdempotencyKey(authorID int64, key string) string {
	return fmt.Sprintf("%d:%s", authorID, key)
}

// publishedAtUnix 用于内存列表排序，空发布时间排到最后。
func publishedAtUnix(video *domainvideo.Video) int64 {
	if video.PublishedAt == nil {
		return 0
	}
	return video.PublishedAt.UnixNano()
}
