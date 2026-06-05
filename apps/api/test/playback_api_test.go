package test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	applicationplayback "GCFeed/internal/application/playback"
	domainplayback "GCFeed/internal/domain/playback"
	infrajwt "GCFeed/internal/infra/jwt"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	interfaceshttpplayback "GCFeed/internal/interfaces/http/playback"

	"github.com/gin-gonic/gin"
)

type playbackConfigAPIResponse struct {
	ID           int64     `json:"id"`
	Platform     string    `json:"platform"`
	NetworkType  string    `json:"network_type"`
	PreloadCount int       `json:"preload_count"`
	BufferMs     int       `json:"buffer_ms"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type preloadVideoAPIResponse struct {
	VideoID  int64  `json:"video_id"`
	MediaURL string `json:"media_url"`
	CoverURL string `json:"cover_url"`
}

type preloadVideosAPIResponse struct {
	Items []preloadVideoAPIResponse `json:"items"`
}

type qosReportAPIResponse struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	VideoID      int64     `json:"video_id"`
	FirstFrameMs *int      `json:"first_frame_ms"`
	StutterCount int       `json:"stutter_count"`
	WatchMs      int       `json:"watch_ms"`
	CreatedAt    time.Time `json:"created_at"`
}

type memoryPlaybackRepo struct {
	mu        sync.Mutex
	nextID    int64
	configs   map[memoryPlaybackConfigKey]*domainplayback.Config
	videos    []*memoryPlaybackVideo
	reports   map[int64]*domainplayback.QoSReport
	byRequest map[memoryPlaybackReportKey]int64
}

type memoryPlaybackConfigKey struct {
	Platform    string
	NetworkType string
}

type memoryPlaybackReportKey struct {
	UserID int64
	Key    string
}

type memoryPlaybackVideo struct {
	VideoID     int64
	MediaURL    string
	CoverURL    string
	PublishedAt time.Time
}

func newMemoryPlaybackRepo() *memoryPlaybackRepo {
	now := time.Now().UTC()
	return &memoryPlaybackRepo{
		nextID: 1,
		configs: map[memoryPlaybackConfigKey]*domainplayback.Config{
			{Platform: "Web", NetworkType: "WiFi"}:                            domainplayback.RestoreConfig(7, "Web", "WiFi", 4, 900, now),
			{Platform: "Android", NetworkType: domainplayback.NetworkDefault}: domainplayback.RestoreConfig(8, "Android", domainplayback.NetworkDefault, 2, 1500, now),
		},
		videos: []*memoryPlaybackVideo{
			{VideoID: 103, MediaURL: "/uploads/103.mp4", CoverURL: "/uploads/103.jpg", PublishedAt: now.Add(3 * time.Minute)},
			{VideoID: 102, MediaURL: "/uploads/102.mp4", CoverURL: "/uploads/102.jpg", PublishedAt: now.Add(2 * time.Minute)},
			{VideoID: 101, MediaURL: "/uploads/101.mp4", CoverURL: "/uploads/101.jpg", PublishedAt: now.Add(time.Minute)},
			{VideoID: 100, MediaURL: "/uploads/100.mp4", CoverURL: "/uploads/100.jpg", PublishedAt: now},
		},
		reports:   map[int64]*domainplayback.QoSReport{},
		byRequest: map[memoryPlaybackReportKey]int64{},
	}
}

func (r *memoryPlaybackRepo) FindConfig(ctx context.Context, platform string, networkType string) (*domainplayback.Config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	config := r.configs[memoryPlaybackConfigKey{Platform: platform, NetworkType: networkType}]
	if config == nil {
		return nil, nil
	}
	cloned := *config
	return &cloned, nil
}

func (r *memoryPlaybackRepo) ListPreloadVideos(ctx context.Context, currentVideoID int64, limit int) ([]*domainplayback.PreloadVideo, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	videos := make([]*memoryPlaybackVideo, 0, len(r.videos))
	videos = append(videos, r.videos...)
	sort.Slice(videos, func(i, j int) bool {
		if videos[i].PublishedAt.Equal(videos[j].PublishedAt) {
			return videos[i].VideoID > videos[j].VideoID
		}
		return videos[i].PublishedAt.After(videos[j].PublishedAt)
	})

	var current *memoryPlaybackVideo
	for _, video := range videos {
		if video.VideoID == currentVideoID {
			current = video
			break
		}
	}

	items := make([]*domainplayback.PreloadVideo, 0, limit)
	for _, video := range videos {
		if current != nil && (video.PublishedAt.After(current.PublishedAt) || video.PublishedAt.Equal(current.PublishedAt) && video.VideoID >= current.VideoID) {
			continue
		}
		items = append(items, domainplayback.RestorePreloadVideo(video.VideoID, video.MediaURL, video.CoverURL))
		if len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *memoryPlaybackRepo) CreateQoSReport(ctx context.Context, report *domainplayback.QoSReport) (*domainplayback.QoSReport, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if report.IdempotencyKey != "" {
		if id, exists := r.byRequest[memoryPlaybackReportKey{UserID: report.UserID, Key: report.IdempotencyKey}]; exists {
			return cloneQoSReport(r.reports[id]), false, nil
		}
	}

	created := cloneQoSReport(report)
	created.ID = r.nextID
	r.nextID++
	if created.CreatedAt.IsZero() {
		created.CreatedAt = time.Now().UTC().Add(time.Duration(created.ID) * time.Millisecond)
	}
	r.reports[created.ID] = created
	if created.IdempotencyKey != "" {
		r.byRequest[memoryPlaybackReportKey{UserID: created.UserID, Key: created.IdempotencyKey}] = created.ID
	}
	return cloneQoSReport(created), true, nil
}

func cloneQoSReport(report *domainplayback.QoSReport) *domainplayback.QoSReport {
	cloned := *report
	if report.FirstFrameMs != nil {
		firstFrameMs := *report.FirstFrameMs
		cloned.FirstFrameMs = &firstFrameMs
	}
	return &cloned
}

func TestPlaybackAPIFlow(t *testing.T) {
	router, jwtManager := newPlaybackRouter(t)
	token := signTestToken(t, jwtManager, 42)

	configResponse := performJSONRequest(router, http.MethodGet, "/api/playback-config?platform=Web&network_type=WiFi", "", token)
	requireStatus(t, configResponse, http.StatusOK)

	var config playbackConfigAPIResponse
	decodeJSON(t, configResponse, &config)
	if config.ID != 7 || config.PreloadCount != 4 || config.BufferMs != 900 {
		t.Fatalf("unexpected playback config: %+v", config)
	}

	defaultConfigResponse := performJSONRequest(router, http.MethodGet, "/api/playback-config?platform=Web&network_type=4G", "", token)
	requireStatus(t, defaultConfigResponse, http.StatusOK)

	decodeJSON(t, defaultConfigResponse, &config)
	if config.ID != 0 || config.PreloadCount != domainplayback.DefaultPreloadCount || config.BufferMs != domainplayback.DefaultBufferMs {
		t.Fatalf("expected default playback config, got %+v", config)
	}

	fallbackConfigResponse := performJSONRequest(router, http.MethodGet, "/api/playback-config?platform=Android&network_type=5G", "", token)
	requireStatus(t, fallbackConfigResponse, http.StatusOK)

	decodeJSON(t, fallbackConfigResponse, &config)
	if config.ID != 8 || config.PreloadCount != 2 || config.BufferMs != 1500 {
		t.Fatalf("expected network fallback playback config, got %+v", config)
	}

	preloadResponse := performJSONRequest(router, http.MethodGet, "/api/preload-videos?current_video_id=103&limit=2", "", token)
	requireStatus(t, preloadResponse, http.StatusOK)

	var preload preloadVideosAPIResponse
	decodeJSON(t, preloadResponse, &preload)
	if len(preload.Items) != 2 || preload.Items[0].VideoID != 102 || preload.Items[1].VideoID != 101 {
		t.Fatalf("unexpected preload items: %+v", preload.Items)
	}

	firstFrameMs := 186
	qosResponse := performPlaybackJSONRequest(
		router,
		http.MethodPost,
		"/api/playback-qos-reports",
		`{"video_id":103,"first_frame_ms":186,"stutter_count":1,"watch_ms":2400}`,
		token,
		"qos-1",
	)
	requireStatus(t, qosResponse, http.StatusCreated)

	var qos qosReportAPIResponse
	decodeJSON(t, qosResponse, &qos)
	if qos.UserID != 42 || qos.VideoID != 103 || qos.FirstFrameMs == nil || *qos.FirstFrameMs != firstFrameMs || qos.StutterCount != 1 {
		t.Fatalf("unexpected qos report: %+v", qos)
	}

	replayResponse := performPlaybackJSONRequest(
		router,
		http.MethodPost,
		"/api/playback-qos-reports",
		`{"video_id":103,"first_frame_ms":999,"stutter_count":9,"watch_ms":9999}`,
		token,
		"qos-1",
	)
	requireStatus(t, replayResponse, http.StatusOK)

	var replay qosReportAPIResponse
	decodeJSON(t, replayResponse, &replay)
	if replay.ID != qos.ID || replay.FirstFrameMs == nil || *replay.FirstFrameMs != firstFrameMs {
		t.Fatalf("expected idempotent qos replay, got %+v", replay)
	}

	internalResponse := performInternalPlaybackRequest(
		router,
		http.MethodPost,
		"/internal/playback-qos-reports",
		`{"user_id":77,"video_id":101,"stutter_count":0,"watch_ms":1200}`,
		testInternalToken,
		"qos-internal-1",
	)
	requireStatus(t, internalResponse, http.StatusCreated)
}

func TestPlaybackAPIValidation(t *testing.T) {
	router, jwtManager := newPlaybackRouter(t)
	token := signTestToken(t, jwtManager, 42)

	requireStatus(t, performJSONRequest(router, http.MethodGet, "/api/playback-config", "", ""), http.StatusUnauthorized)
	requireStatus(t, performJSONRequest(router, http.MethodGet, "/api/preload-videos", "", ""), http.StatusUnauthorized)
	requireStatus(t, performPlaybackJSONRequest(router, http.MethodPost, "/api/playback-qos-reports", `{"video_id":103}`, "", ""), http.StatusUnauthorized)

	requireStatus(t, performJSONRequest(router, http.MethodGet, "/api/playback-config?platform="+strings.Repeat("x", 17), "", token), http.StatusBadRequest)
	requireStatus(t, performJSONRequest(router, http.MethodGet, "/api/preload-videos?limit=0", "", token), http.StatusBadRequest)
	requireStatus(t, performJSONRequest(router, http.MethodGet, "/api/preload-videos?current_video_id=bad", "", token), http.StatusBadRequest)
	requireStatus(t, performPlaybackJSONRequest(router, http.MethodPost, "/api/playback-qos-reports", `{"video_id":0}`, token, ""), http.StatusBadRequest)
	requireStatus(t, performPlaybackJSONRequest(router, http.MethodPost, "/api/playback-qos-reports", `{"video_id":103,"first_frame_ms":-1}`, token, ""), http.StatusBadRequest)
	requireStatus(t, performPlaybackJSONRequest(router, http.MethodPost, "/api/playback-qos-reports", `{"video_id":103,"stutter_count":-1}`, token, ""), http.StatusBadRequest)
	requireStatus(t, performPlaybackJSONRequest(router, http.MethodPost, "/api/playback-qos-reports", `{"video_id":103,"watch_ms":-1}`, token, ""), http.StatusBadRequest)
	requireStatus(t, performInternalPlaybackRequest(router, http.MethodPost, "/internal/playback-qos-reports", `{"user_id":77,"video_id":101}`, "", ""), http.StatusUnauthorized)
	requireStatus(t, performInternalPlaybackRequest(router, http.MethodPost, "/internal/playback-qos-reports", `{"user_id":77,"video_id":101}`, "wrong-token", ""), http.StatusUnauthorized)
	requireStatus(t, performInternalPlaybackRequest(router, http.MethodPost, "/internal/playback-qos-reports", `{"user_id":-1,"video_id":101}`, testInternalToken, ""), http.StatusBadRequest)
}

func newPlaybackRouter(t *testing.T) (*gin.Engine, *infrajwt.Manager) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	repo := newMemoryPlaybackRepo()
	service := applicationplayback.New(repo)
	handler := interfaceshttpplayback.New(service)
	jwtManager, err := infrajwt.NewManager("test-secret", "15m")
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}

	router := gin.New()
	authMiddleware := interfaceshttpmiddleware.NewJWTAuth(jwtManager)
	api := router.Group("/api", authMiddleware)
	api.GET("/playback-config", handler.GetConfig)
	api.GET("/preload-videos", handler.ListPreloadVideos)
	api.POST("/playback-qos-reports", handler.CreateQoSReport)
	router.POST("/internal/playback-qos-reports", interfaceshttpmiddleware.NewInternalTokenAuth(testInternalToken), handler.CreateInternalQoSReport)

	return router, jwtManager
}

func performPlaybackJSONRequest(router *gin.Engine, method, path, body, accessToken, idempotencyKey string) *httptest.ResponseRecorder {
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

func performInternalPlaybackRequest(router *gin.Engine, method, path, body, internalToken, idempotencyKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if internalToken != "" {
		req.Header.Set("X-Internal-Token", internalToken)
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}
