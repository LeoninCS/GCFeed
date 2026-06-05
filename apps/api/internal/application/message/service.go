package applicationmessage

import (
	domainmessage "GCFeed/internal/domain/message"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const defaultMessageLimit = 20

var ErrLoadMessageFailed = errors.New("failed to load message")
var ErrSaveMessageFailed = errors.New("failed to save message")
var ErrUpdateMessageFailed = errors.New("failed to update message")

type Service struct {
	repo domainmessage.Repository
}

type CreateResult struct {
	Message *domainmessage.Message
	Created bool
}

type ListResult struct {
	Items      []*domainmessage.Message
	NextCursor string
	HasMore    bool
}

type UnreadStat struct {
	UnreadCount int
}

type MarkReadResult struct {
	UpdatedCount int
}

type cursorPayload struct {
	CreatedAt string `json:"created_at"`
	MessageID int64  `json:"message_id"`
}

func New(repo domainmessage.Repository) *Service {
	return &Service{repo: repo}
}

// CreateFromEvent 将内部事件转换成用户消息，eventID/idempotencyKey 命中时返回既有消息。
func (s *Service) CreateFromEvent(ctx context.Context, userID int64, messageType string, title string, content string, eventID string, idempotencyKey string) (*CreateResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > domainmessage.MaxIdempotencyKeyLength {
		return nil, domainmessage.ErrIdempotencyKeyTooLong
	}

	message, err := domainmessage.New(userID, messageType, title, content, eventID)
	if err != nil {
		return nil, err
	}

	created, inserted, err := s.repo.Create(ctx, message, idempotencyKey)
	if err != nil {
		return nil, ErrSaveMessageFailed
	}
	return &CreateResult{Message: created, Created: inserted}, nil
}

// List 查询当前用户消息列表，使用游标分页。
func (s *Service) List(ctx context.Context, userID int64, cursor string, limit int) (*ListResult, error) {
	if userID <= 0 {
		return nil, domainmessage.ErrInvalidUserID
	}

	parsedCursor, err := parseCursor(cursor)
	if err != nil {
		return nil, err
	}
	limit = normalizeLimit(limit)

	items, err := s.repo.ListByUser(ctx, userID, parsedCursor, limit+1)
	if err != nil {
		return nil, ErrLoadMessageFailed
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor := ""
	if len(items) > 0 {
		nextCursor = encodeCursor(&domainmessage.Cursor{
			CreatedAt: items[len(items)-1].CreatedAt,
			MessageID: items[len(items)-1].ID,
		})
	}

	return &ListResult{Items: items, NextCursor: nextCursor, HasMore: hasMore}, nil
}

// CountUnread 查询当前用户未读数。
func (s *Service) CountUnread(ctx context.Context, userID int64) (*UnreadStat, error) {
	if userID <= 0 {
		return nil, domainmessage.ErrInvalidUserID
	}

	count, err := s.repo.CountUnread(ctx, userID)
	if err != nil {
		return nil, ErrLoadMessageFailed
	}
	return &UnreadStat{UnreadCount: count}, nil
}

// MarkRead 将当前用户消息标记为已读，空 messageIDs 表示全部已读。
func (s *Service) MarkRead(ctx context.Context, userID int64, messageIDs []int64) (*MarkReadResult, error) {
	if userID <= 0 {
		return nil, domainmessage.ErrInvalidUserID
	}

	ids := make([]int64, 0, len(messageIDs))
	seen := map[int64]struct{}{}
	for _, id := range messageIDs {
		if id <= 0 {
			return nil, domainmessage.ErrInvalidMessageID
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	count, err := s.repo.MarkRead(ctx, userID, ids)
	if err != nil {
		return nil, ErrUpdateMessageFailed
	}
	return &MarkReadResult{UpdatedCount: count}, nil
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultMessageLimit
	}
	if limit > domainmessage.MaxLimit {
		return domainmessage.MaxLimit
	}
	return limit
}

func parseCursor(raw string) (*domainmessage.Cursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	content, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		content, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, domainmessage.ErrInvalidCursor
		}
	}

	var payload cursorPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, domainmessage.ErrInvalidCursor
	}

	createdAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.CreatedAt))
	if err != nil || payload.MessageID <= 0 {
		return nil, domainmessage.ErrInvalidCursor
	}

	return &domainmessage.Cursor{CreatedAt: createdAt, MessageID: payload.MessageID}, nil
}

func encodeCursor(cursor *domainmessage.Cursor) string {
	if cursor == nil || cursor.MessageID <= 0 || cursor.CreatedAt.IsZero() {
		return ""
	}

	content, err := json.Marshal(cursorPayload{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		MessageID: cursor.MessageID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(content)
}
