package applicationfeed

import (
	domainfeed "GCFeed/internal/domain/feed"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const defaultFeedLimit = 10

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

// TimelineFeedResult 兼容原有时间线用例命名。
type TimelineFeedResult = FeedResult

// Strategy 定义单个 Feed 场景的读取策略。
type Strategy interface {
	Scene() domainfeed.Scene
	List(ctx context.Context, req FeedRequest) (*FeedResult, error)
}

// TimelineStrategy 复用现有时间线查询能力。
type TimelineStrategy struct {
	scene domainfeed.Scene
	repo  domainfeed.Repository
}

type timelineCursorPayload struct {
	PublishedAt string `json:"published_at"`
	VideoID     int64  `json:"video_id"`
}

// WithStrategy 注册一个额外 Feed 策略。
func WithStrategy(strategy Strategy) Option {
	return func(s *Service) {
		s.RegisterStrategy(strategy)
	}
}

// New 注入 Feed 仓储并注册默认时间线策略。
func New(repo domainfeed.Repository, options ...Option) *Service {
	service := &Service{
		strategies:   map[domainfeed.Scene]Strategy{},
		defaultScene: domainfeed.DefaultScene,
	}
	service.RegisterStrategy(NewTimelineStrategy(domainfeed.SceneTimeline, repo))
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
func (s *Service) GetTimelineFeed(ctx context.Context, cursor string, limit int) (*TimelineFeedResult, error) {
	return s.GetFeed(ctx, FeedRequest{
		Scene:  domainfeed.SceneTimeline,
		Cursor: cursor,
		Limit:  limit,
	})
}

// NewTimelineStrategy 创建一个时间线排序策略，可作为 latest 等场景的基础实现。
func NewTimelineStrategy(scene domainfeed.Scene, repo domainfeed.Repository) *TimelineStrategy {
	return &TimelineStrategy{
		scene: domainfeed.NormalizeScene(scene),
		repo:  repo,
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

	items, err := s.repo.ListTimelineFeed(ctx, parsedCursor, limit+1)
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

	return &FeedResult{
		Scene:      s.scene,
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// RefreshTimelineFeed 从第一页重新加载 Feed，适合下拉刷新场景。
func (s *Service) RefreshTimelineFeed(ctx context.Context, limit int) (*TimelineFeedResult, error) {
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
