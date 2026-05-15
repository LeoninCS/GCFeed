package applicationfeed

import (
	domainfeed "GCFeed/internal/domain/feed"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const defaultFeedLimit = 10
const timelineFirstPageCacheTTL = 5 * time.Second
const timelinePageCacheTTL = 45 * time.Second
const feedCardCacheTTL = 15 * time.Minute
const feedStatCacheTTL = 15 * time.Second

var ErrLoadFeedFailed = errors.New("failed to load feed")

// Service 通过 scene 策略注册表分发不同 Feed 场景。
type Service struct {
	strategies   map[domainfeed.Scene]Strategy
	defaultScene domainfeed.Scene
}

// Option 用于在装配阶段注册额外 Feed 策略。
type Option func(*Service)

// FeedRequest 是所有 Feed 场景共用的查询参数。
type FeedRequest struct {
	Scene         domainfeed.Scene
	Cursor        string
	Limit         int
	ViewerID      int64
	ClientContext map[string]string
}

// FeedResult 是游标分页结果，NextCursor 供客户端请求下一页。
type FeedResult struct {
	Scene      domainfeed.Scene
	Items      []*domainfeed.FeedItem
	NextCursor string
	HasMore    bool
}

// FeedPage 是页缓存中的轻量结果，卡片和计数会在组装阶段批量读取。
type FeedPage struct {
	Scene      domainfeed.Scene
	Items      []*domainfeed.FeedPageItem
	NextCursor string
	HasMore    bool
}

// FeedCache 定义 Feed 页、卡片和计数缓存能力，Redis 实现在基础设施层提供。
type FeedCache interface {
	GetPage(ctx context.Context, key string) (*FeedPage, bool, error)
	SetPage(ctx context.Context, key string, page *FeedPage, ttl time.Duration) error
	GetCards(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedCard, error)
	SetCards(ctx context.Context, cards map[int64]*domainfeed.FeedCard, ttl time.Duration) error
	GetStats(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedStat, error)
	SetStats(ctx context.Context, stats map[int64]*domainfeed.FeedStat, ttl time.Duration) error
	ListHotWindowPage(ctx context.Context, windowEnd time.Time, offset int, limit int) ([]*domainfeed.FeedPageItem, error)
}

type FollowingIndexCache interface {
	ListFollowingIndexPage(ctx context.Context, viewerID int64, authorIDs []int64, cursor *domainfeed.TimelineCursor, limit int) ([]*domainfeed.FeedPageItem, bool, error)
}

// Strategy 定义单个 Feed 场景的读取策略。
type Strategy interface {
	Scene() domainfeed.Scene
	List(ctx context.Context, req FeedRequest) (*FeedResult, error)
}

// TimelineStrategy 复用现有时间线查询能力。
type TimelineStrategy struct {
	scene        domainfeed.Scene
	repo         domainfeed.Repository
	cache        FeedCache
	firstPageTTL time.Duration
	pageTTL      time.Duration
	group        singleflight.Group
}

// HotStrategy 使用互动热度读取热榜 Feed。
type HotStrategy struct {
	repo  domainfeed.Repository
	cache FeedCache
}

// FollowingStrategy 使用推拉混合模式读取关注流。
type FollowingStrategy struct {
	repo           domainfeed.Repository
	cache          FeedCache
	followingIndex FollowingIndexCache
}

type timelineCursorPayload struct {
	PublishedAt string `json:"published_at"`
	VideoID     int64  `json:"video_id"`
}

type hotCursorPayload struct {
	HotScore    int    `json:"hot_score"`
	PublishedAt string `json:"published_at"`
	VideoID     int64  `json:"video_id"`
	WindowEnd   string `json:"window_end,omitempty"`
	Offset      int    `json:"offset,omitempty"`
}

// WithStrategy 注册一个额外 Feed 策略。
func WithStrategy(strategy Strategy) Option {
	return func(s *Service) {
		s.RegisterStrategy(strategy)
	}
}

// WithFeedCache 为 Feed 页、卡片和计数启用读缓存。
func WithFeedCache(cache FeedCache) Option {
	return func(s *Service) {
		for _, strategy := range s.strategies {
			switch typed := strategy.(type) {
			case *TimelineStrategy:
				typed.cache = cache
			case *HotStrategy:
				typed.cache = cache
			case *FollowingStrategy:
				typed.cache = cache
				if index, ok := cache.(FollowingIndexCache); ok {
					typed.followingIndex = index
				}
			}
		}
	}
}

// New 注入 Feed 仓储并注册默认时间线策略。
func New(repo domainfeed.Repository, options ...Option) *Service {
	service := &Service{
		strategies:   map[domainfeed.Scene]Strategy{},
		defaultScene: domainfeed.DefaultScene,
	}
	service.RegisterStrategy(NewTimelineStrategy(domainfeed.SceneTimeline, repo))
	service.RegisterStrategy(NewHotStrategy(repo))
	service.RegisterStrategy(NewFollowingStrategy(repo))
	for _, option := range options {
		option(service)
	}
	return service
}

// RegisterStrategy 把 scene 和具体策略绑定，新增 Feed 类型时在装配层调用。
func (s *Service) RegisterStrategy(strategy Strategy) {
	if strategy == nil {
		return
	}
	scene := domainfeed.NormalizeScene(strategy.Scene())
	s.strategies[scene] = strategy
}

// GetFeed 根据 scene 选择策略并返回分页结果。
func (s *Service) GetFeed(ctx context.Context, req FeedRequest) (*FeedResult, error) {
	req.Scene = domainfeed.NormalizeScene(req.Scene)
	if req.Scene == "" {
		req.Scene = s.defaultScene
	}

	strategy, ok := s.strategies[req.Scene]
	if !ok {
		return nil, domainfeed.ErrUnsupportedScene
	}
	return strategy.List(ctx, req)
}

// GetTimelineFeed 使用 cursor+limit 读取时间线 Feed。
func (s *Service) GetTimelineFeed(ctx context.Context, cursor string, limit int) (*FeedResult, error) {
	return s.GetFeed(ctx, FeedRequest{
		Scene:  domainfeed.SceneTimeline,
		Cursor: cursor,
		Limit:  limit,
	})
}

// NewTimelineStrategy 创建一个时间线排序策略，可作为 latest 等场景的基础实现。
func NewTimelineStrategy(scene domainfeed.Scene, repo domainfeed.Repository) *TimelineStrategy {
	return &TimelineStrategy{
		scene:        domainfeed.NormalizeScene(scene),
		repo:         repo,
		firstPageTTL: timelineFirstPageCacheTTL,
		pageTTL:      timelinePageCacheTTL,
	}
}

// Scene 返回当前策略绑定的 Feed 场景。
func (s *TimelineStrategy) Scene() domainfeed.Scene {
	return s.scene
}

// List 使用 cursor+limit 读取时间线 Feed。
func (s *TimelineStrategy) List(ctx context.Context, req FeedRequest) (*FeedResult, error) {
	parsedCursor, err := parseTimelineCursor(req.Cursor)
	if err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)

	page, err := loadFeedPage(ctx, s.cache, s.scene, req.Cursor, limit, s.firstPageTTL, s.pageTTL, &s.group, func() (*FeedPage, error) {
		return s.listPageFromRepo(ctx, parsedCursor, limit)
	})
	if err != nil {
		return nil, err
	}

	items, err := assembleFeedItems(ctx, s.repo, s.cache, page.Items)
	if err != nil {
		return nil, ErrLoadFeedFailed
	}

	return &FeedResult{
		Scene:      s.scene,
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}, nil
}

func (s *TimelineStrategy) listPageFromRepo(ctx context.Context, parsedCursor *domainfeed.TimelineCursor, limit int) (*FeedPage, error) {
	items, err := s.repo.ListTimelinePage(ctx, parsedCursor, limit+1)
	if err != nil {
		return nil, ErrLoadFeedFailed
	}

	// limit+1 是常见分页技巧：多取一条即可判断后面还有没有数据。
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor := ""
	if len(items) > 0 {
		// 下一页从当前页最后一个元素之后开始，游标保存排序所需的两个字段。
		nextCursor = encodeTimelineCursor(&domainfeed.TimelineCursor{
			PublishedAt: items[len(items)-1].PublishedAt,
			VideoID:     items[len(items)-1].VideoID,
		})
	}

	return &FeedPage{
		Scene:      s.scene,
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// NewHotStrategy 创建热榜排序策略。
func NewHotStrategy(repo domainfeed.Repository) *HotStrategy {
	return &HotStrategy{repo: repo}
}

// Scene 返回热榜场景。
func (s *HotStrategy) Scene() domainfeed.Scene {
	return domainfeed.SceneHot
}

// List 读取热榜；Redis 场景使用最近一小时分钟桶，基础场景使用仓储累计热度。
func (s *HotStrategy) List(ctx context.Context, req FeedRequest) (*FeedResult, error) {
	parsedCursor, err := parseHotCursor(req.Cursor)
	if err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)

	var page *FeedPage
	if s.cache != nil {
		if strings.TrimSpace(req.Cursor) != "" && (parsedCursor == nil || parsedCursor.WindowEnd.IsZero()) {
			return nil, domainfeed.ErrInvalidCursor
		}
		page, err = s.listPageFromHotWindow(ctx, parsedCursor, limit)
	} else {
		page, err = s.listPageFromRepo(ctx, parsedCursor, limit)
	}
	if err != nil {
		return nil, err
	}
	items, err := assembleFeedItems(ctx, s.repo, s.cache, page.Items)
	if err != nil {
		return nil, ErrLoadFeedFailed
	}

	return &FeedResult{
		Scene:      domainfeed.SceneHot,
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}, nil
}

func (s *HotStrategy) listPageFromHotWindow(ctx context.Context, parsedCursor *domainfeed.HotCursor, limit int) (*FeedPage, error) {
	windowEnd := time.Now().UTC().Truncate(time.Minute)
	offset := 0
	if parsedCursor != nil && !parsedCursor.WindowEnd.IsZero() {
		windowEnd = parsedCursor.WindowEnd.UTC().Truncate(time.Minute)
		offset = parsedCursor.Offset
	}

	items, err := s.cache.ListHotWindowPage(ctx, windowEnd, offset, limit+1)
	if err != nil {
		return nil, ErrLoadFeedFailed
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor := ""
	if len(items) > 0 {
		nextCursor = encodeHotWindowCursor(&domainfeed.HotCursor{
			WindowEnd: windowEnd,
			Offset:    offset + len(items),
		})
	}

	return &FeedPage{
		Scene:      domainfeed.SceneHot,
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *HotStrategy) listPageFromRepo(ctx context.Context, parsedCursor *domainfeed.HotCursor, limit int) (*FeedPage, error) {
	items, err := s.repo.ListHotPage(ctx, parsedCursor, limit+1)
	if err != nil {
		return nil, ErrLoadFeedFailed
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor := ""
	if len(items) > 0 {
		last := items[len(items)-1]
		nextCursor = encodeHotCursor(&domainfeed.HotCursor{
			HotScore:    last.HotScore,
			PublishedAt: last.PublishedAt,
			VideoID:     last.VideoID,
		})
	}

	return &FeedPage{
		Scene:      domainfeed.SceneHot,
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// NewFollowingStrategy 创建关注流推拉混合策略。
func NewFollowingStrategy(repo domainfeed.Repository) *FollowingStrategy {
	return &FollowingStrategy{repo: repo}
}

// Scene 返回关注流场景。
func (s *FollowingStrategy) Scene() domainfeed.Scene {
	return domainfeed.SceneFollowing
}

// List 根据当前登录用户读取关注流。
func (s *FollowingStrategy) List(ctx context.Context, req FeedRequest) (*FeedResult, error) {
	if req.ViewerID <= 0 {
		return nil, domainfeed.ErrViewerRequired
	}
	parsedCursor, err := parseTimelineCursor(req.Cursor)
	if err != nil {
		return nil, err
	}
	limit := normalizeLimit(req.Limit)

	page, err := s.listPageFromRepo(ctx, req.ViewerID, parsedCursor, limit)
	if err != nil {
		return nil, err
	}
	items, err := assembleFeedItems(ctx, s.repo, s.cache, page.Items)
	if err != nil {
		return nil, ErrLoadFeedFailed
	}

	return &FeedResult{
		Scene:      domainfeed.SceneFollowing,
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}, nil
}

func (s *FollowingStrategy) listPageFromRepo(ctx context.Context, viewerID int64, parsedCursor *domainfeed.TimelineCursor, limit int) (*FeedPage, error) {
	items, err := s.listFollowingItems(ctx, viewerID, parsedCursor, limit+1)
	if err != nil {
		return nil, ErrLoadFeedFailed
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor := ""
	if len(items) > 0 {
		nextCursor = encodeTimelineCursor(&domainfeed.TimelineCursor{
			PublishedAt: items[len(items)-1].PublishedAt,
			VideoID:     items[len(items)-1].VideoID,
		})
	}

	return &FeedPage{
		Scene:      domainfeed.SceneFollowing,
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *FollowingStrategy) listFollowingItems(ctx context.Context, viewerID int64, parsedCursor *domainfeed.TimelineCursor, limit int) ([]*domainfeed.FeedPageItem, error) {
	if s.followingIndex != nil {
		authorIDs, err := s.repo.ListFollowingPullAuthorIDs(ctx, viewerID)
		if err != nil {
			return nil, err
		}
		items, ok, err := s.followingIndex.ListFollowingIndexPage(ctx, viewerID, authorIDs, parsedCursor, limit)
		if err != nil {
			return nil, err
		}
		if ok {
			return items, nil
		}
	}
	return s.repo.ListFollowingPage(ctx, viewerID, parsedCursor, limit)
}

// RefreshFeed 从第一页重新加载默认 Feed，适合下拉刷新场景。
func (s *Service) RefreshFeed(ctx context.Context, limit int) (*FeedResult, error) {
	return s.GetTimelineFeed(ctx, "", limit)
}

// normalizeLimit 统一默认页大小和最大页大小。
func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultFeedLimit
	}
	if limit > domainfeed.MaxLimit {
		return domainfeed.MaxLimit
	}
	return limit
}

func loadFeedPage(ctx context.Context, cache FeedCache, scene domainfeed.Scene, cursor string, limit int, firstPageTTL time.Duration, pageTTL time.Duration, group *singleflight.Group, load func() (*FeedPage, error)) (*FeedPage, error) {
	if cache == nil || group == nil {
		return load()
	}

	cacheKey := feedPageCacheKey(scene, cursor, limit)
	if page, ok, err := cache.GetPage(ctx, cacheKey); err == nil && ok {
		return page, nil
	}

	value, err, _ := group.Do(cacheKey, func() (any, error) {
		if page, ok, err := cache.GetPage(ctx, cacheKey); err == nil && ok {
			return page, nil
		}
		page, err := load()
		if err != nil {
			return nil, err
		}
		_ = cache.SetPage(ctx, cacheKey, page, feedPageCacheTTL(cursor, cacheKey, firstPageTTL, pageTTL))
		return page, nil
	})
	if err != nil {
		return nil, err
	}
	page, ok := value.(*FeedPage)
	if !ok {
		return nil, ErrLoadFeedFailed
	}
	return page, nil
}

func assembleFeedItems(ctx context.Context, repo domainfeed.Repository, cache FeedCache, pageItems []*domainfeed.FeedPageItem) ([]*domainfeed.FeedItem, error) {
	videoIDs := feedPageVideoIDs(pageItems)
	if len(videoIDs) == 0 {
		return []*domainfeed.FeedItem{}, nil
	}

	cards := map[int64]*domainfeed.FeedCard{}
	stats := map[int64]*domainfeed.FeedStat{}
	if cache != nil {
		if cachedCards, err := cache.GetCards(ctx, videoIDs); err == nil {
			cards = cachedCards
		}
		if cachedStats, err := cache.GetStats(ctx, videoIDs); err == nil {
			stats = cachedStats
		}
	}

	missingCardIDs := missingCardIDs(videoIDs, cards)
	if len(missingCardIDs) > 0 {
		loadedCards, err := repo.BatchGetFeedCards(ctx, missingCardIDs)
		if err != nil {
			return nil, err
		}
		mergeCards(cards, loadedCards)
		if cache != nil {
			_ = cache.SetCards(ctx, loadedCards, feedCardCacheTTL)
		}
	}

	missingStatIDs := missingStatIDs(videoIDs, stats)
	if len(missingStatIDs) > 0 {
		loadedStats, err := repo.BatchGetFeedStats(ctx, missingStatIDs)
		if err != nil {
			return nil, err
		}
		mergeStats(stats, loadedStats)
		if cache != nil {
			_ = cache.SetStats(ctx, loadedStats, feedStatCacheTTL)
		}
	}

	items := make([]*domainfeed.FeedItem, 0, len(pageItems))
	for _, pageItem := range pageItems {
		if pageItem == nil {
			continue
		}
		card, ok := cards[pageItem.VideoID]
		if !ok || card == nil {
			continue
		}
		stat := stats[pageItem.VideoID]
		if stat == nil {
			stat = &domainfeed.FeedStat{VideoID: pageItem.VideoID}
		}
		publishedAt := pageItem.PublishedAt
		if publishedAt.IsZero() {
			publishedAt = card.PublishedAt
		}
		item := domainfeed.RestoreFeedItem(
			card.VideoID,
			card.AuthorID,
			card.AuthorNickname,
			card.AuthorAvatarURL,
			card.Title,
			card.Description,
			card.MediaURL,
			card.CoverURL,
			stat.LikeCount,
			stat.CommentCount,
			stat.FavoriteCount,
			publishedAt,
		)
		item.HotScore = pageItem.HotScore
		items = append(items, item)
	}
	return items, nil
}

func feedPageVideoIDs(items []*domainfeed.FeedPageItem) []int64 {
	videoIDs := make([]int64, 0, len(items))
	seen := map[int64]struct{}{}
	for _, item := range items {
		if item == nil || item.VideoID <= 0 {
			continue
		}
		if _, ok := seen[item.VideoID]; ok {
			continue
		}
		seen[item.VideoID] = struct{}{}
		videoIDs = append(videoIDs, item.VideoID)
	}
	return videoIDs
}

func missingCardIDs(videoIDs []int64, cards map[int64]*domainfeed.FeedCard) []int64 {
	missing := make([]int64, 0)
	for _, videoID := range videoIDs {
		if cards[videoID] == nil {
			missing = append(missing, videoID)
		}
	}
	return missing
}

func missingStatIDs(videoIDs []int64, stats map[int64]*domainfeed.FeedStat) []int64 {
	missing := make([]int64, 0)
	for _, videoID := range videoIDs {
		if stats[videoID] == nil {
			missing = append(missing, videoID)
		}
	}
	return missing
}

func mergeCards(target map[int64]*domainfeed.FeedCard, source map[int64]*domainfeed.FeedCard) {
	for videoID, card := range source {
		if card != nil {
			target[videoID] = card
		}
	}
}

func mergeStats(target map[int64]*domainfeed.FeedStat, source map[int64]*domainfeed.FeedStat) {
	for videoID, stat := range source {
		if stat != nil {
			target[videoID] = stat
		}
	}
}

func feedPageCacheKey(scene domainfeed.Scene, cursor string, limit int) string {
	scene = domainfeed.NormalizeScene(scene)
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return fmt.Sprintf("feed:page:v1:%s:limit:%d:first", scene, limit)
	}

	sum := sha1.Sum([]byte(cursor))
	return fmt.Sprintf("feed:page:v1:%s:limit:%d:cursor:%s", scene, limit, hex.EncodeToString(sum[:]))
}

func feedPageCacheTTL(cursor string, cacheKey string, firstPageTTL time.Duration, pageTTL time.Duration) time.Duration {
	ttl := pageTTL
	if strings.TrimSpace(cursor) == "" {
		ttl = firstPageTTL
	}
	if ttl <= 0 {
		return 0
	}

	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(cacheKey))
	jitterPercent := 10 + int(hasher.Sum32()%11)
	return ttl + time.Duration(jitterPercent)*ttl/100
}

// parseTimelineCursor 将客户端传回的字符串游标解析成领域游标。
func parseTimelineCursor(raw string) (*domainfeed.TimelineCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	content, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		content, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, domainfeed.ErrInvalidCursor
		}
	}

	var payload timelineCursorPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, domainfeed.ErrInvalidCursor
	}

	publishedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.PublishedAt))
	if err != nil || payload.VideoID <= 0 {
		return nil, domainfeed.ErrInvalidCursor
	}

	return &domainfeed.TimelineCursor{
		PublishedAt: publishedAt,
		VideoID:     payload.VideoID,
	}, nil
}

// parseHotCursor 将客户端传回的热榜游标解析成领域游标。
func parseHotCursor(raw string) (*domainfeed.HotCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	content, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		content, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, domainfeed.ErrInvalidCursor
		}
	}

	var payload hotCursorPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, domainfeed.ErrInvalidCursor
	}

	if strings.TrimSpace(payload.WindowEnd) != "" {
		windowEnd, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.WindowEnd))
		if err != nil || payload.Offset < 0 {
			return nil, domainfeed.ErrInvalidCursor
		}
		return &domainfeed.HotCursor{
			WindowEnd: windowEnd,
			Offset:    payload.Offset,
		}, nil
	}

	publishedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.PublishedAt))
	if err != nil || payload.VideoID <= 0 {
		return nil, domainfeed.ErrInvalidCursor
	}

	return &domainfeed.HotCursor{
		HotScore:    payload.HotScore,
		PublishedAt: publishedAt,
		VideoID:     payload.VideoID,
	}, nil
}

// encodeTimelineCursor 把排序字段编码成 URL 安全的游标字符串。
func encodeTimelineCursor(cursor *domainfeed.TimelineCursor) string {
	if cursor == nil || cursor.VideoID <= 0 || cursor.PublishedAt.IsZero() {
		return ""
	}

	content, err := json.Marshal(timelineCursorPayload{
		PublishedAt: cursor.PublishedAt.UTC().Format(time.RFC3339Nano),
		VideoID:     cursor.VideoID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(content)
}

// encodeHotWindowCursor 把热榜滑动窗口位置编码成 URL 安全的游标字符串。
func encodeHotWindowCursor(cursor *domainfeed.HotCursor) string {
	if cursor == nil || cursor.WindowEnd.IsZero() || cursor.Offset < 0 {
		return ""
	}

	content, err := json.Marshal(hotCursorPayload{
		WindowEnd: cursor.WindowEnd.UTC().Format(time.RFC3339Nano),
		Offset:    cursor.Offset,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(content)
}

// encodeHotCursor 把热榜排序字段编码成 URL 安全的游标字符串。
func encodeHotCursor(cursor *domainfeed.HotCursor) string {
	if cursor == nil || cursor.VideoID <= 0 || cursor.PublishedAt.IsZero() {
		return ""
	}

	content, err := json.Marshal(hotCursorPayload{
		HotScore:    cursor.HotScore,
		PublishedAt: cursor.PublishedAt.UTC().Format(time.RFC3339Nano),
		VideoID:     cursor.VideoID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(content)
}
