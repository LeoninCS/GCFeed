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
var ErrSaveViewEventFailed = errors.New("failed to save view event")
var ErrLoadViewEventFailed = errors.New("failed to load view event")

type Service struct {
	repo domainfeed.Repository
}

type TimeFeedResult struct {
	Items      []*domainfeed.FeedItem
	NextCursor string
	HasMore    bool
}

type ViewEventResult struct {
	Event   *domainfeed.ViewEvent
	Created bool
}

type timeCursorPayload struct {
	PublishedAt string `json:"published_at"`
	VideoID     int64  `json:"video_id"`
}

func NewService(repo domainfeed.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetTimeFeed(ctx context.Context, cursor string, limit int) (*TimeFeedResult, error) {
	parsedCursor, err := parseTimeCursor(cursor)
	if err != nil {
		return nil, err
	}
	limit = normalizeLimit(limit)

	items, err := s.repo.ListTimeFeed(ctx, parsedCursor, limit+1)
	if err != nil {
		return nil, ErrLoadFeedFailed
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor := ""
	if len(items) > 0 {
		nextCursor = encodeTimeCursor(&domainfeed.TimeCursor{
			PublishedAt: items[len(items)-1].PublishedAt,
			VideoID:     items[len(items)-1].VideoID,
		})
	}

	return &TimeFeedResult{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

func (s *Service) RefreshTimeFeed(ctx context.Context, limit int) (*TimeFeedResult, error) {
	return s.GetTimeFeed(ctx, "", limit)
}

func (s *Service) ReportViewEvent(ctx context.Context, userID *int64, visitorID string, videoID int64, eventType string, watchMS int, idempotencyKey string) (*ViewEventResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > domainfeed.MaxIdempotencyKeyLength {
		return nil, domainfeed.ErrIdempotencyKeyTooLong
	}

	if idempotencyKey != "" {
		existing, err := s.repo.FindViewEventByIdempotencyKey(ctx, idempotencyKey)
		if err == nil {
			return &ViewEventResult{Event: existing, Created: false}, nil
		}
		if !errors.Is(err, domainfeed.ErrViewEventNotFound) {
			return nil, ErrLoadViewEventFailed
		}
	}

	event, err := domainfeed.NewViewEvent(userID, visitorID, videoID, eventType, watchMS, idempotencyKey)
	if err != nil {
		return nil, err
	}

	if err := s.repo.SaveViewEvent(ctx, event); err != nil {
		if idempotencyKey != "" && errors.Is(err, domainfeed.ErrDuplicateIdempotencyKey) {
			existing, loadErr := s.repo.FindViewEventByIdempotencyKey(ctx, idempotencyKey)
			if loadErr == nil {
				return &ViewEventResult{Event: existing, Created: false}, nil
			}
			return nil, ErrLoadViewEventFailed
		}
		return nil, ErrSaveViewEventFailed
	}

	return &ViewEventResult{Event: event, Created: true}, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultFeedLimit
	}
	if limit > domainfeed.MaxLimit {
		return domainfeed.MaxLimit
	}
	return limit
}

func parseTimeCursor(raw string) (*domainfeed.TimeCursor, error) {
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

	var payload timeCursorPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, domainfeed.ErrInvalidCursor
	}

	publishedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.PublishedAt))
	if err != nil || payload.VideoID <= 0 {
		return nil, domainfeed.ErrInvalidCursor
	}

	return &domainfeed.TimeCursor{
		PublishedAt: publishedAt,
		VideoID:     payload.VideoID,
	}, nil
}

func encodeTimeCursor(cursor *domainfeed.TimeCursor) string {
	if cursor == nil || cursor.VideoID <= 0 || cursor.PublishedAt.IsZero() {
		return ""
	}

	content, err := json.Marshal(timeCursorPayload{
		PublishedAt: cursor.PublishedAt.UTC().Format(time.RFC3339Nano),
		VideoID:     cursor.VideoID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(content)
}
