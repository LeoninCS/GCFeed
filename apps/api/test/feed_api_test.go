package test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	applicationfeed "GCFeed/internal/application/feed"
	domainfeed "GCFeed/internal/domain/feed"
	interfaceshttpfeed "GCFeed/internal/interfaces/http/feed"

	"github.com/gin-gonic/gin"
)

type feedAPIResponse struct {
	Items      []feedItemAPIResponse `json:"items"`
	NextCursor string                `json:"next_cursor"`
	HasMore    bool                  `json:"has_more"`
}

type feedItemAPIResponse struct {
	VideoID       int64     `json:"video_id"`
	AuthorID      int64     `json:"author_id"`
	Title         string    `json:"title"`
	MediaURL      string    `json:"media_url"`
	CoverURL      string    `json:"cover_url"`
	LikeCount     int       `json:"like_count"`
	CommentCount  int       `json:"comment_count"`
	FavoriteCount int       `json:"favorite_count"`
	PublishedAt   time.Time `json:"published_at"`
}

type viewEventAPIResponse struct {
	ID        int64     `json:"id"`
	VisitorID string    `json:"visitor_id"`
	VideoID   int64     `json:"video_id"`
	EventType string    `json:"event_type"`
	WatchMS   int       `json:"watch_ms"`
	CreatedAt time.Time `json:"created_at"`
}

type memoryFeedRepo struct {
	mu          sync.Mutex
	items       []*domainfeed.FeedItem
	nextEventID int64
	eventsByID  map[int64]*domainfeed.ViewEvent
	eventsByKey map[string]int64
}

func newMemoryFeedRepo(items []*domainfeed.FeedItem) *memoryFeedRepo {
	return &memoryFeedRepo{
		items:       items,
		nextEventID: 1,
		eventsByID:  map[int64]*domainfeed.ViewEvent{},
		eventsByKey: map[string]int64{},
	}
}

func (r *memoryFeedRepo) ListTimeFeed(ctx context.Context, cursor *domainfeed.TimeCursor, limit int) ([]*domainfeed.FeedItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*domainfeed.FeedItem, 0, len(r.items))
	for _, item := range r.items {
		if cursor == nil || item.PublishedAt.Before(cursor.PublishedAt) || (item.PublishedAt.Equal(cursor.PublishedAt) && item.VideoID < cursor.VideoID) {
			items = append(items, cloneFeedItem(item))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].PublishedAt.Equal(items[j].PublishedAt) {
			return items[i].VideoID > items[j].VideoID
		}
		return items[i].PublishedAt.After(items[j].PublishedAt)
	})

	if limit > len(items) {
		limit = len(items)
	}
	return items[:limit], nil
}

func (r *memoryFeedRepo) SaveViewEvent(ctx context.Context, event *domainfeed.ViewEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if event.IdempotencyKey != "" {
		if _, exists := r.eventsByKey[event.IdempotencyKey]; exists {
			return domainfeed.ErrDuplicateIdempotencyKey
		}
	}

	event.ID = r.nextEventID
	r.nextEventID++
	event.CreatedAt = time.Now()
	r.eventsByID[event.ID] = cloneViewEvent(event)
	if event.IdempotencyKey != "" {
		r.eventsByKey[event.IdempotencyKey] = event.ID
	}
	return nil
}

func (r *memoryFeedRepo) FindViewEventByIdempotencyKey(ctx context.Context, key string) (*domainfeed.ViewEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, exists := r.eventsByKey[key]
	if !exists {
		return nil, domainfeed.ErrViewEventNotFound
	}
	return cloneViewEvent(r.eventsByID[id]), nil
}

func TestFeedAPIFlow(t *testing.T) {
	router := newFeedRouter(seedFeedItems())

	firstPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed/time?limit=2", "", "")
	requireStatus(t, firstPageResponse, http.StatusOK)

	var firstPage feedAPIResponse
	decodeJSON(t, firstPageResponse, &firstPage)
	if len(firstPage.Items) != 2 || firstPage.Items[0].VideoID != 3 || firstPage.Items[1].VideoID != 2 {
		t.Fatalf("unexpected first page response: %+v", firstPage)
	}
	if firstPage.NextCursor == "" || !firstPage.HasMore {
		t.Fatalf("unexpected first page cursor: %+v", firstPage)
	}

	secondPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed/time?cursor="+firstPage.NextCursor+"&limit=2", "", "")
	requireStatus(t, secondPageResponse, http.StatusOK)

	var secondPage feedAPIResponse
	decodeJSON(t, secondPageResponse, &secondPage)
	if len(secondPage.Items) != 1 || secondPage.Items[0].VideoID != 1 || secondPage.HasMore {
		t.Fatalf("unexpected second page response: %+v", secondPage)
	}

	refreshResponse := performJSONRequest(router, http.MethodGet, "/api/feed/refresh?limit=1", "", "")
	requireStatus(t, refreshResponse, http.StatusOK)

	var refresh feedAPIResponse
	decodeJSON(t, refreshResponse, &refresh)
	if len(refresh.Items) != 1 || refresh.Items[0].VideoID != 3 || !refresh.HasMore {
		t.Fatalf("unexpected refresh response: %+v", refresh)
	}

	eventResponse := performFeedJSONRequest(
		router,
		http.MethodPost,
		"/api/feed/view-events",
		`{"visitor_id":"visitor-001","video_id":3,"event_type":"view","watch_ms":3000}`,
		"event-1",
	)
	requireStatus(t, eventResponse, http.StatusCreated)

	var event viewEventAPIResponse
	decodeJSON(t, eventResponse, &event)
	if event.ID == 0 || event.VisitorID != "visitor-001" || event.VideoID != 3 || event.EventType != domainfeed.EventTypeView || event.WatchMS != 3000 {
		t.Fatalf("unexpected event response: %+v", event)
	}

	replayResponse := performFeedJSONRequest(
		router,
		http.MethodPost,
		"/api/feed/view-events",
		`{"visitor_id":"visitor-002","video_id":2,"event_type":"COMPLETE","watch_ms":5000}`,
		"event-1",
	)
	requireStatus(t, replayResponse, http.StatusOK)

	var replayed viewEventAPIResponse
	decodeJSON(t, replayResponse, &replayed)
	if replayed.ID != event.ID || replayed.VideoID != event.VideoID || replayed.VisitorID != event.VisitorID {
		t.Fatalf("unexpected replay response: %+v", replayed)
	}
}

func TestFeedAPIValidation(t *testing.T) {
	router := newFeedRouter(seedFeedItems())

	badLimitResponse := performJSONRequest(router, http.MethodGet, "/api/feed/time?limit=0", "", "")
	requireStatus(t, badLimitResponse, http.StatusBadRequest)

	badCursorResponse := performJSONRequest(router, http.MethodGet, "/api/feed/time?cursor=bad-cursor", "", "")
	requireStatus(t, badCursorResponse, http.StatusBadRequest)

	emptyVideoResponse := performFeedJSONRequest(
		router,
		http.MethodPost,
		"/api/feed/view-events",
		`{"visitor_id":"visitor-001","event_type":"VIEW","watch_ms":3000}`,
		"",
	)
	requireStatus(t, emptyVideoResponse, http.StatusBadRequest)

	badEventTypeResponse := performFeedJSONRequest(
		router,
		http.MethodPost,
		"/api/feed/view-events",
		`{"visitor_id":"visitor-001","video_id":3,"event_type":"SKIP","watch_ms":3000}`,
		"",
	)
	requireStatus(t, badEventTypeResponse, http.StatusBadRequest)

	badWatchResponse := performFeedJSONRequest(
		router,
		http.MethodPost,
		"/api/feed/view-events",
		`{"visitor_id":"visitor-001","video_id":3,"event_type":"VIEW","watch_ms":-1}`,
		"",
	)
	requireStatus(t, badWatchResponse, http.StatusBadRequest)
}

func newFeedRouter(items []*domainfeed.FeedItem) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	repo := newMemoryFeedRepo(items)
	service := applicationfeed.NewService(repo)
	handler := interfaceshttpfeed.NewHandler(service)

	api := router.Group("/api")
	feed := api.Group("/feed")
	feed.GET("/time", handler.Time)
	feed.GET("/refresh", handler.Refresh)
	feed.POST("/view-events", handler.ReportViewEvent)

	return router
}

func performFeedJSONRequest(router *gin.Engine, method, path, body, idempotencyKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func seedFeedItems() []*domainfeed.FeedItem {
	base := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	return []*domainfeed.FeedItem{
		domainfeed.RestoreFeedItem(1, 10, "old video", "https://example.com/1.mp4", "https://example.com/1.jpg", 1, 2, 3, base.Add(-2*time.Hour)),
		domainfeed.RestoreFeedItem(2, 20, "middle video", "https://example.com/2.mp4", "https://example.com/2.jpg", 4, 5, 6, base.Add(-1*time.Hour)),
		domainfeed.RestoreFeedItem(3, 30, "new video", "https://example.com/3.mp4", "https://example.com/3.jpg", 7, 8, 9, base),
	}
}

func cloneFeedItem(item *domainfeed.FeedItem) *domainfeed.FeedItem {
	cloned := *item
	return &cloned
}

func cloneViewEvent(event *domainfeed.ViewEvent) *domainfeed.ViewEvent {
	cloned := *event
	if event.UserID != nil {
		userID := *event.UserID
		cloned.UserID = &userID
	}
	return &cloned
}
