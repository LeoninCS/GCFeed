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
	Scene      string                `json:"scene"`
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

// memoryFeedRepo 是 Feed 测试用内存仓储，模拟时间线排序和游标分页。
type memoryFeedRepo struct {
	mu            sync.Mutex
	items         []*domainfeed.FeedItem
	timelineCalls int
}

type memoryFeedCache struct {
	mu    sync.Mutex
	items map[string]*applicationfeed.FeedResult
}

func newMemoryFeedRepo(items []*domainfeed.FeedItem) *memoryFeedRepo {
	return &memoryFeedRepo{items: items}
}

func newMemoryFeedCache() *memoryFeedCache {
	return &memoryFeedCache{items: map[string]*applicationfeed.FeedResult{}}
}

func (r *memoryFeedRepo) TimelineCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.timelineCalls
}

// ListTimelineFeed 模拟真实仓储的 published_at DESC, video_id DESC 排序。
func (r *memoryFeedRepo) ListTimelineFeed(ctx context.Context, cursor *domainfeed.TimelineCursor, limit int) ([]*domainfeed.FeedItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.timelineCalls++

	items := make([]*domainfeed.FeedItem, 0, len(r.items))
	for _, item := range r.items {
		// cursor 代表上一页最后一条数据，下一页从它之后开始。
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

func (c *memoryFeedCache) Get(ctx context.Context, key string) (*applicationfeed.FeedResult, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	result, ok := c.items[key]
	return result, ok, nil
}

func (c *memoryFeedCache) Set(ctx context.Context, key string, result *applicationfeed.FeedResult, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = result
	return nil
}

// ListHotFeed 模拟真实仓储的 hot_score DESC, published_at DESC, video_id DESC 排序。
func (r *memoryFeedRepo) ListHotFeed(ctx context.Context, cursor *domainfeed.HotCursor, limit int) ([]*domainfeed.FeedItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	items := make([]*domainfeed.FeedItem, 0, len(r.items))
	for _, item := range r.items {
		if cursor == nil ||
			item.HotScore < cursor.HotScore ||
			(item.HotScore == cursor.HotScore && item.PublishedAt.Before(cursor.PublishedAt)) ||
			(item.HotScore == cursor.HotScore && item.PublishedAt.Equal(cursor.PublishedAt) && item.VideoID < cursor.VideoID) {
			items = append(items, cloneFeedItem(item))
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].HotScore == items[j].HotScore {
			if items[i].PublishedAt.Equal(items[j].PublishedAt) {
				return items[i].VideoID > items[j].VideoID
			}
			return items[i].PublishedAt.After(items[j].PublishedAt)
		}
		return items[i].HotScore > items[j].HotScore
	})

	if limit > len(items) {
		limit = len(items)
	}
	return items[:limit], nil
}

// TestFeedAPIFlow 覆盖 Feed 首页、下一页游标和刷新读取。
func TestFeedAPIFlow(t *testing.T) {
	router := newFeedRouter(seedFeedItems())

	firstPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?limit=2", "", "")
	requireStatus(t, firstPageResponse, http.StatusOK)

	var firstPage feedAPIResponse
	decodeJSON(t, firstPageResponse, &firstPage)
	if firstPage.Scene != string(domainfeed.SceneTimeline) {
		t.Fatalf("unexpected feed scene: %+v", firstPage)
	}
	if len(firstPage.Items) != 2 || firstPage.Items[0].VideoID != 3 || firstPage.Items[1].VideoID != 2 {
		t.Fatalf("unexpected first page response: %+v", firstPage)
	}
	if firstPage.Items[0].AuthorNickname != "new author" || firstPage.Items[0].AuthorAvatarURL != "https://example.com/avatar-3.jpg" || firstPage.Items[0].Description != "new description" {
		t.Fatalf("unexpected first page author response: %+v", firstPage.Items[0])
	}
	if firstPage.NextCursor == "" || !firstPage.HasMore {
		t.Fatalf("unexpected first page cursor: %+v", firstPage)
	}

	secondPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?cursor="+firstPage.NextCursor+"&limit=2", "", "")
	requireStatus(t, secondPageResponse, http.StatusOK)

	var secondPage feedAPIResponse
	decodeJSON(t, secondPageResponse, &secondPage)
	if len(secondPage.Items) != 1 || secondPage.Items[0].VideoID != 1 || secondPage.HasMore {
		t.Fatalf("unexpected second page response: %+v", secondPage)
	}

	refreshResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?limit=1", "", "")
	requireStatus(t, refreshResponse, http.StatusOK)

	var refresh feedAPIResponse
	decodeJSON(t, refreshResponse, &refresh)
	if len(refresh.Items) != 1 || refresh.Items[0].VideoID != 3 || !refresh.HasMore {
		t.Fatalf("unexpected refresh response: %+v", refresh)
	}
}

// TestFeedSceneQuery 覆盖 scene 参数和复杂查询入口。
func TestFeedSceneQuery(t *testing.T) {
	router := newFeedRouter(seedFeedItems())

	sceneResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=timeline&limit=1", "", "")
	requireStatus(t, sceneResponse, http.StatusOK)

	var sceneFeed feedAPIResponse
	decodeJSON(t, sceneResponse, &sceneFeed)
	if sceneFeed.Scene != string(domainfeed.SceneTimeline) || len(sceneFeed.Items) != 1 || sceneFeed.Items[0].VideoID != 3 {
		t.Fatalf("unexpected scene response: %+v", sceneFeed)
	}

	queryResponse := performJSONRequest(
		router,
		http.MethodPost,
		"/api/feed-queries",
		`{"scene":"timeline","limit":2,"context":{"device":"ios","experiment":"rank_v1"}}`,
		"",
	)
	requireStatus(t, queryResponse, http.StatusOK)

	var queryFeed feedAPIResponse
	decodeJSON(t, queryResponse, &queryFeed)
	if queryFeed.Scene != string(domainfeed.SceneTimeline) || len(queryFeed.Items) != 2 {
		t.Fatalf("unexpected query response: %+v", queryFeed)
	}

	unknownSceneResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=following&limit=1", "", "")
	requireStatus(t, unknownSceneResponse, http.StatusBadRequest)
}

// TestHotFeedScene 覆盖热榜 Feed 排序和游标分页。
func TestHotFeedScene(t *testing.T) {
	router := newFeedRouter(seedHotFeedItems())

	firstPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=hot&limit=2", "", "")
	requireStatus(t, firstPageResponse, http.StatusOK)

	var firstPage feedAPIResponse
	decodeJSON(t, firstPageResponse, &firstPage)
	if firstPage.Scene != string(domainfeed.SceneHot) {
		t.Fatalf("unexpected hot feed scene: %+v", firstPage)
	}
	if len(firstPage.Items) != 2 || firstPage.Items[0].VideoID != 1 || firstPage.Items[1].VideoID != 2 {
		t.Fatalf("unexpected hot first page response: %+v", firstPage)
	}
	if firstPage.NextCursor == "" || !firstPage.HasMore {
		t.Fatalf("unexpected hot first page cursor: %+v", firstPage)
	}

	secondPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=hot&cursor="+firstPage.NextCursor+"&limit=2", "", "")
	requireStatus(t, secondPageResponse, http.StatusOK)

	var secondPage feedAPIResponse
	decodeJSON(t, secondPageResponse, &secondPage)
	if len(secondPage.Items) != 1 || secondPage.Items[0].VideoID != 3 || secondPage.HasMore {
		t.Fatalf("unexpected hot second page response: %+v", secondPage)
	}
}

// TestTimelineFeedCache 覆盖 timeline Feed 缓存命中。
func TestTimelineFeedCache(t *testing.T) {
	repo := newMemoryFeedRepo(seedFeedItems())
	cache := newMemoryFeedCache()
	router := newFeedRouterWithService(applicationfeed.New(repo, applicationfeed.WithTimelineCache(cache)))

	firstResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=timeline&limit=2", "", "")
	requireStatus(t, firstResponse, http.StatusOK)
	if repo.TimelineCalls() != 1 {
		t.Fatalf("unexpected timeline repo calls after first request: %d", repo.TimelineCalls())
	}

	secondResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=timeline&limit=2", "", "")
	requireStatus(t, secondResponse, http.StatusOK)
	if repo.TimelineCalls() != 1 {
		t.Fatalf("unexpected timeline repo calls after cached request: %d", repo.TimelineCalls())
	}

	var secondPage feedAPIResponse
	decodeJSON(t, secondResponse, &secondPage)
	if len(secondPage.Items) != 2 || secondPage.Items[0].VideoID != 3 || secondPage.Items[1].VideoID != 2 {
		t.Fatalf("unexpected cached timeline response: %+v", secondPage)
	}
}

// TestFeedAPIValidation 覆盖 limit 和 cursor 参数校验。
func TestFeedAPIValidation(t *testing.T) {
	router := newFeedRouter(seedFeedItems())

	badLimitResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?limit=0", "", "")
	requireStatus(t, badLimitResponse, http.StatusBadRequest)

	badCursorResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?cursor=bad-cursor", "", "")
	requireStatus(t, badCursorResponse, http.StatusBadRequest)
}

// newFeedRouter 只装配 Feed 路由，测试时无需数据库。
func newFeedRouter(items []*domainfeed.FeedItem) *gin.Engine {
	return newFeedRouterWithService(applicationfeed.New(newMemoryFeedRepo(items)))
}

func newFeedRouterWithService(service *applicationfeed.Service) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	handler := interfaceshttpfeed.New(service)

	api := router.Group("/api")
	api.GET("/feed-items", handler.Timeline)
	api.POST("/feed-queries", handler.Query)

	return router
}

// seedFeedItems 准备三条不同发布时间的视频，用于验证排序和分页。
func seedFeedItems() []*domainfeed.FeedItem {
	base := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	return []*domainfeed.FeedItem{
		domainfeed.RestoreFeedItem(1, 10, "old author", "https://example.com/avatar-1.jpg", "old video", "old description", "https://example.com/1.mp4", "https://example.com/1.jpg", 1, 2, 3, base.Add(-2*time.Hour)),
		domainfeed.RestoreFeedItem(2, 20, "middle author", "https://example.com/avatar-2.jpg", "middle video", "middle description", "https://example.com/2.mp4", "https://example.com/2.jpg", 4, 5, 6, base.Add(-1*time.Hour)),
		domainfeed.RestoreFeedItem(3, 30, "new author", "https://example.com/avatar-3.jpg", "new video", "new description", "https://example.com/3.mp4", "https://example.com/3.jpg", 7, 8, 9, base),
	}
}

// seedHotFeedItems 准备热度和发布时间错开的数据，用于验证热榜排序。
func seedHotFeedItems() []*domainfeed.FeedItem {
	base := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	return []*domainfeed.FeedItem{
		domainfeed.RestoreFeedItem(1, 10, "old author", "https://example.com/avatar-1.jpg", "old hot video", "old hot description", "https://example.com/1.mp4", "https://example.com/1.jpg", 20, 30, 10, base.Add(-2*time.Hour)),
		domainfeed.RestoreFeedItem(2, 20, "middle author", "https://example.com/avatar-2.jpg", "middle warm video", "middle warm description", "https://example.com/2.mp4", "https://example.com/2.jpg", 10, 1, 0, base.Add(-1*time.Hour)),
		domainfeed.RestoreFeedItem(3, 30, "new author", "https://example.com/avatar-3.jpg", "new quiet video", "new quiet description", "https://example.com/3.mp4", "https://example.com/3.jpg", 0, 0, 0, base),
	}
}

// cloneFeedItem 返回 FeedItem 副本，隔离仓储内部数据。
func cloneFeedItem(item *domainfeed.FeedItem) *domainfeed.FeedItem {
	cloned := *item
	return &cloned
}
