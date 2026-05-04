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

const defaultFeedLimit = 20

var ErrLoadFeedFailed = errors.New("failed to load feed")

// Service 提供 Feed 读取用例，目前按发布时间倒序返回时间线。
type Service struct {
	repo domainfeed.Repository
}

// TimelineFeedResult 是游标分页结果，NextCursor 供客户端请求下一页。
type TimelineFeedResult struct {
	Items      []*domainfeed.FeedItem
	NextCursor string
	HasMore    bool
}

type timelineCursorPayload struct {
	PublishedAt string `json:"published_at"`
	VideoID     int64  `json:"video_id"`
}

// New 注入 Feed 仓储，应用层依赖领域仓储接口。
func New(repo domainfeed.Repository) *Service {
	return &Service{repo: repo}
}

// GetTimelineFeed 使用 cursor+limit 读取时间线 Feed。
func (s *Service) GetTimelineFeed(ctx context.Context, cursor string, limit int) (*TimelineFeedResult, error) {
	parsedCursor, err := parseTimelineCursor(cursor)
	if err != nil {
		return nil, err
	}
	limit = normalizeLimit(limit)

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

	return &TimelineFeedResult{
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
