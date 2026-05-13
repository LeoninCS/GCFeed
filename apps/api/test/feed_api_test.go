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
	hotCalls      int
	cardCalls     int
	statCalls     int
}

type memoryFeedCache struct {
	mu       sync.Mutex
	pages    map[string]*applicationfeed.FeedPage
	cards    map[int64]*domainfeed.FeedCard
	stats    map[int64]*domainfeed.FeedStat
	hotItems []*domainfeed.FeedPageItem
}

func newMemoryFeedRepo(items []*domainfeed.FeedItem) *memoryFeedRepo {
	return &memoryFeedRepo{items: items}
}

func newMemoryFeedCache() *memoryFeedCache {
	return &memoryFeedCache{
		pages: map[string]*applicationfeed.FeedPage{},
		cards: map[int64]*domainfeed.FeedCard{},
		stats: map[int64]*domainfeed.FeedStat{},
	}
}

func (r *memoryFeedRepo) TimelineCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.timelineCalls
}

func (r *memoryFeedRepo) CardCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cardCalls
}

func (r *memoryFeedRepo) StatCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.statCalls
}

func (r *memoryFeedRepo) HotCalls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.hotCalls
}

// ListTimelinePage 模拟真实仓储的 published_at DESC, video_id DESC 排序。
func (r *memoryFeedRepo) ListTimelinePage(ctx context.Context, cursor *domainfeed.TimelineCursor, limit int) ([]*domainfeed.FeedPageItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.timelineCalls++

	items := make([]*domainfeed.FeedPageItem, 0, len(r.items))
	for _, item := range r.items {
		// cursor 代表上一页最后一条数据，下一页从它之后开始。
		if cursor == nil || item.PublishedAt.Before(cursor.PublishedAt) || (item.PublishedAt.Equal(cursor.PublishedAt) && item.VideoID < cursor.VideoID) {
			items = append(items, feedPageItemFromFeedItem(item))
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

func (c *memoryFeedCache) GetPage(ctx context.Context, key string) (*applicationfeed.FeedPage, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	page, ok := c.pages[key]
	if !ok {
		return nil, false, nil
	}
	cloned := *page
	cloned.Items = cloneFeedPageItems(page.Items)
	return &cloned, true, nil
}

func (c *memoryFeedCache) SetPage(ctx context.Context, key string, page *applicationfeed.FeedPage, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cloned := *page
	cloned.Items = cloneFeedPageItems(page.Items)
	c.pages[key] = &cloned
	return nil
}

func (c *memoryFeedCache) GetCards(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedCard, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cards := map[int64]*domainfeed.FeedCard{}
	for _, videoID := range videoIDs {
		if card := c.cards[videoID]; card != nil {
			cloned := *card
			cards[videoID] = &cloned
		}
	}
	return cards, nil
}

func (c *memoryFeedCache) SetCards(ctx context.Context, cards map[int64]*domainfeed.FeedCard, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for videoID, card := range cards {
		if card == nil {
			continue
		}
		cloned := *card
		c.cards[videoID] = &cloned
	}
	return nil
}

func (c *memoryFeedCache) GetStats(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedStat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	stats := map[int64]*domainfeed.FeedStat{}
	for _, videoID := range videoIDs {
		if stat := c.stats[videoID]; stat != nil {
			cloned := *stat
			stats[videoID] = &cloned
		}
	}
	return stats, nil
}

func (c *memoryFeedCache) SetStats(ctx context.Context, stats map[int64]*domainfeed.FeedStat, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for videoID, stat := range stats {
		if stat == nil {
			continue
		}
		cloned := *stat
		c.stats[videoID] = &cloned
	}
	return nil
}

func (c *memoryFeedCache) SetHotWindowItems(items []*domainfeed.FeedPageItem) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.hotItems = cloneFeedPageItems(items)
}

func (c *memoryFeedCache) ListHotWindowPage(ctx context.Context, windowEnd time.Time, offset int, limit int) ([]*domainfeed.FeedPageItem, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	items := cloneFeedPageItems(c.hotItems)
	if offset < 0 {
		offset = 0
	}
	if offset >= len(items) || limit <= 0 {
		return []*domainfeed.FeedPageItem{}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], nil
}

// ListHotPage 模拟真实仓储的 hot_score DESC, published_at DESC, video_id DESC 排序。
func (r *memoryFeedRepo) ListHotPage(ctx context.Context, cursor *domainfeed.HotCursor, limit int) ([]*domainfeed.FeedPageItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.hotCalls++

	items := make([]*domainfeed.FeedPageItem, 0, len(r.items))
	for _, item := range r.items {
		if cursor == nil ||
			item.HotScore < cursor.HotScore ||
			(item.HotScore == cursor.HotScore && item.PublishedAt.Before(cursor.PublishedAt)) ||
			(item.HotScore == cursor.HotScore && item.PublishedAt.Equal(cursor.PublishedAt) && item.VideoID < cursor.VideoID) {
			items = append(items, feedPageItemFromFeedItem(item))
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

func (r *memoryFeedRepo) BatchGetFeedCards(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedCard, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cardCalls++

	cards := map[int64]*domainfeed.FeedCard{}
	wanted := int64Set(videoIDs)
	for _, item := range r.items {
		if _, ok := wanted[item.VideoID]; ok {
			cards[item.VideoID] = feedCardFromFeedItem(item)
		}
	}
	return cards, nil
}

func (r *memoryFeedRepo) BatchGetFeedStats(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedStat, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.statCalls++

	stats := map[int64]*domainfeed.FeedStat{}
	wanted := int64Set(videoIDs)
	for _, item := range r.items {
		if _, ok := wanted[item.VideoID]; ok {
			stats[item.VideoID] = feedStatFromFeedItem(item)
		}
	}
	return stats, nil
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

// TestHotFeedWindowCache 覆盖热榜从 Redis 窗口页读取并继续批量组装卡片。
func TestHotFeedWindowCache(t *testing.T) {
	repo := newMemoryFeedRepo(seedHotFeedItems())
	cache := newMemoryFeedCache()
	cache.SetHotWindowItems([]*domainfeed.FeedPageItem{
		{VideoID: 2, HotScore: 80},
		{VideoID: 1, HotScore: 60},
		{VideoID: 3, HotScore: 20},
	})
	router := newFeedRouterWithService(applicationfeed.New(repo, applicationfeed.WithFeedCache(cache)))

	firstPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=hot&limit=2", "", "")
	requireStatus(t, firstPageResponse, http.StatusOK)

	var firstPage feedAPIResponse
	decodeJSON(t, firstPageResponse, &firstPage)
	if len(firstPage.Items) != 2 || firstPage.Items[0].VideoID != 2 || firstPage.Items[1].VideoID != 1 || !firstPage.HasMore {
		t.Fatalf("unexpected hot window first page response: %+v", firstPage)
	}
	if firstPage.NextCursor == "" {
		t.Fatalf("unexpected hot window cursor: %+v", firstPage)
	}
	if repo.HotCalls() != 0 {
		t.Fatalf("unexpected hot repo calls: %d", repo.HotCalls())
	}

	secondPageResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=hot&cursor="+firstPage.NextCursor+"&limit=2", "", "")
	requireStatus(t, secondPageResponse, http.StatusOK)

	var secondPage feedAPIResponse
	decodeJSON(t, secondPageResponse, &secondPage)
	if len(secondPage.Items) != 1 || secondPage.Items[0].VideoID != 3 || secondPage.HasMore {
		t.Fatalf("unexpected hot window second page response: %+v", secondPage)
	}
}

// TestTimelineFeedCache 覆盖 timeline Feed 缓存命中。
func TestTimelineFeedCache(t *testing.T) {
	repo := newMemoryFeedRepo(seedFeedItems())
	cache := newMemoryFeedCache()
	router := newFeedRouterWithService(applicationfeed.New(repo, applicationfeed.WithFeedCache(cache)))

	firstResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=timeline&limit=2", "", "")
	requireStatus(t, firstResponse, http.StatusOK)
	if repo.TimelineCalls() != 1 {
		t.Fatalf("unexpected timeline repo calls after first request: %d", repo.TimelineCalls())
	}
	if repo.CardCalls() != 1 || repo.StatCalls() != 1 {
		t.Fatalf("unexpected card/stat repo calls after first request: card=%d stat=%d", repo.CardCalls(), repo.StatCalls())
	}

	secondResponse := performJSONRequest(router, http.MethodGet, "/api/feed-items?scene=timeline&limit=2", "", "")
	requireStatus(t, secondResponse, http.StatusOK)
	if repo.TimelineCalls() != 1 {
		t.Fatalf("unexpected timeline repo calls after cached request: %d", repo.TimelineCalls())
	}
	if repo.CardCalls() != 1 || repo.StatCalls() != 1 {
		t.Fatalf("unexpected card/stat repo calls after cached request: card=%d stat=%d", repo.CardCalls(), repo.StatCalls())
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
	api.GET("/feed-items", handler.ListFeedItems)
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

func feedPageItemFromFeedItem(item *domainfeed.FeedItem) *domainfeed.FeedPageItem {
	return &domainfeed.FeedPageItem{
		VideoID:     item.VideoID,
		PublishedAt: item.PublishedAt,
		HotScore:    item.HotScore,
	}
}

func feedCardFromFeedItem(item *domainfeed.FeedItem) *domainfeed.FeedCard {
	return &domainfeed.FeedCard{
		VideoID:         item.VideoID,
		AuthorID:        item.AuthorID,
		AuthorNickname:  item.AuthorNickname,
		AuthorAvatarURL: item.AuthorAvatarURL,
		Title:           item.Title,
		Description:     item.Description,
		MediaURL:        item.MediaURL,
		CoverURL:        item.CoverURL,
		PublishedAt:     item.PublishedAt,
	}
}

func feedStatFromFeedItem(item *domainfeed.FeedItem) *domainfeed.FeedStat {
	return &domainfeed.FeedStat{
		VideoID:       item.VideoID,
		LikeCount:     item.LikeCount,
		CommentCount:  item.CommentCount,
		FavoriteCount: item.FavoriteCount,
	}
}

func cloneFeedPageItems(items []*domainfeed.FeedPageItem) []*domainfeed.FeedPageItem {
	cloned := make([]*domainfeed.FeedPageItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		value := *item
		cloned = append(cloned, &value)
	}
	return cloned
}

func int64Set(values []int64) map[int64]struct{} {
	set := map[int64]struct{}{}
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
