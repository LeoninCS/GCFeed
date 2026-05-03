package test

import (
	"context"
	"net/http"
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
	VideoID         int64     `json:"video_id"`
	AuthorID        int64     `json:"author_id"`
	AuthorNickname  string    `json:"author_nickname"`
	AuthorAvatarURL string    `json:"author_avatar_url"`
	Title           string    `json:"title"`
	Description     string    `json:"description"`
	MediaURL        string    `json:"media_url"`
	CoverURL        string    `json:"cover_url"`
	LikeCount       int       `json:"like_count"`
	CommentCount    int       `json:"comment_count"`
	FavoriteCount   int       `json:"favorite_count"`
	PublishedAt     time.Time `json:"published_at"`
}

type memoryFeedRepo struct {
	mu    sync.Mutex
	items []*domainfeed.FeedItem
}

func newMemoryFeedRepo(items []*domainfeed.FeedItem) *memoryFeedRepo {
	return &memoryFeedRepo{items: items}
}

func (r *memoryFeedRepo) ListTimelineFeed(ctx context.Context, cursor *domainfeed.TimelineCursor, limit int) ([]*domainfeed.FeedItem, error) {
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

func TestFeedAPIFlow(t *testing.T) {
	router := newFeedRouter(seedFeedItems())

	firstPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed/timeline?limit=2", "", "")
	requireStatus(t, firstPageResponse, http.StatusOK)

	var firstPage feedAPIResponse
	decodeJSON(t, firstPageResponse, &firstPage)
	if len(firstPage.Items) != 2 || firstPage.Items[0].VideoID != 3 || firstPage.Items[1].VideoID != 2 {
		t.Fatalf("unexpected first page response: %+v", firstPage)
	}
	if firstPage.Items[0].AuthorNickname != "new author" || firstPage.Items[0].AuthorAvatarURL != "https://example.com/avatar-3.jpg" || firstPage.Items[0].Description != "new description" {
		t.Fatalf("unexpected first page author response: %+v", firstPage.Items[0])
	}
	if firstPage.NextCursor == "" || !firstPage.HasMore {
		t.Fatalf("unexpected first page cursor: %+v", firstPage)
	}

	secondPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed/timeline?cursor="+firstPage.NextCursor+"&limit=2", "", "")
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
}

func TestFeedAPIValidation(t *testing.T) {
	router := newFeedRouter(seedFeedItems())

	badLimitResponse := performJSONRequest(router, http.MethodGet, "/api/feed/timeline?limit=0", "", "")
	requireStatus(t, badLimitResponse, http.StatusBadRequest)

	badCursorResponse := performJSONRequest(router, http.MethodGet, "/api/feed/timeline?cursor=bad-cursor", "", "")
	requireStatus(t, badCursorResponse, http.StatusBadRequest)
}

func newFeedRouter(items []*domainfeed.FeedItem) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	repo := newMemoryFeedRepo(items)
	service := applicationfeed.NewService(repo)
	handler := interfaceshttpfeed.NewHandler(service)

	api := router.Group("/api")
	feed := api.Group("/feed")
	feed.GET("/timeline", handler.Timeline)
	feed.GET("/refresh", handler.Refresh)

	return router
}

func seedFeedItems() []*domainfeed.FeedItem {
	base := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	return []*domainfeed.FeedItem{
		domainfeed.RestoreFeedItem(1, 10, "old author", "https://example.com/avatar-1.jpg", "old video", "old description", "https://example.com/1.mp4", "https://example.com/1.jpg", 1, 2, 3, base.Add(-2*time.Hour)),
		domainfeed.RestoreFeedItem(2, 20, "middle author", "https://example.com/avatar-2.jpg", "middle video", "middle description", "https://example.com/2.mp4", "https://example.com/2.jpg", 4, 5, 6, base.Add(-1*time.Hour)),
		domainfeed.RestoreFeedItem(3, 30, "new author", "https://example.com/avatar-3.jpg", "new video", "new description", "https://example.com/3.mp4", "https://example.com/3.jpg", 7, 8, 9, base),
	}
}

func cloneFeedItem(item *domainfeed.FeedItem) *domainfeed.FeedItem {
	cloned := *item
	return &cloned
}
