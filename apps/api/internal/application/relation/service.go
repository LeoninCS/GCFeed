package applicationrelation

import (
	domainfeed "GCFeed/internal/domain/feed"
	domainmessage "GCFeed/internal/domain/message"
	domainrelation "GCFeed/internal/domain/relation"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const defaultListLimit = 20
const defaultBackfillVideoLimit = 100
const defaultBackfillInboxMaxLen = 1000

var ErrLoadRelationFailed = errors.New("failed to load relation")
var ErrUpdateRelationFailed = errors.New("failed to update relation")
var ErrBackfillFollowFeedFailed = errors.New("failed to backfill follow feed")

// Service 编排用户关系用例：关注、取关、关注列表和粉丝列表。
type Service struct {
	repo          domainrelation.Repository
	backfiller    FollowFeedBackfiller
	messageWriter MessageWriter
}

type FollowFeedBackfiller interface {
	CountFollowers(ctx context.Context, authorID int64) (int, error)
	ListAuthorRecentVideos(ctx context.Context, authorID int64, limit int) ([]*domainfeed.FeedPageItem, error)
	AddInboxItems(ctx context.Context, authorID int64, userIDs []int64, item *domainfeed.FeedPageItem, maxLen int64) error
}

type Option func(*Service)

// MessageWriter 写入关注触发的站内消息。
type MessageWriter interface {
	CreateFromEvent(ctx context.Context, userID int64, messageType string, title string, content string, eventID string, idempotencyKey string) (any, error)
}

// ActorMessageWriter 可在关注消息里携带触发用户资料。
type ActorMessageWriter interface {
	CreateFromActorEvent(ctx context.Context, userID int64, messageType string, title string, content string, eventID string, idempotencyKey string, actorID int64, actorNickname string, actorAvatarURL string) (any, error)
}

// FollowResult 是关注或取关后的关系状态和计数。
type FollowResult struct {
	UserID         int64
	TargetUserID   int64
	Status         int
	Following      bool
	FollowingCount int
	FollowerCount  int
}

// ListResult 是关注列表或粉丝列表的游标分页结果。
type ListResult struct {
	Items      []*domainrelation.UserItem
	NextCursor string
	HasMore    bool
}

type listCursorPayload struct {
	FollowedAt string `json:"followed_at"`
	UserID     int64  `json:"user_id"`
}

// New 创建关系应用服务。
func New(repo domainrelation.Repository, options ...Option) *Service {
	service := &Service{repo: repo}
	for _, option := range options {
		option(service)
	}
	return service
}

// WithFollowFeedBackfiller 为关注成功后的关注流索引启用回填。
func WithFollowFeedBackfiller(backfiller FollowFeedBackfiller) Option {
	return func(s *Service) {
		s.backfiller = backfiller
	}
}

// WithMessageWriter 为关注成功后的通知写入启用消息中心。
func WithMessageWriter(writer MessageWriter) Option {
	return func(s *Service) {
		s.messageWriter = writer
	}
}

// Follow 设置当前用户关注目标用户。
func (s *Service) Follow(ctx context.Context, userID int64, targetUserID int64, idempotencyKey string) (*FollowResult, error) {
	return s.setFollow(ctx, userID, targetUserID, true, idempotencyKey)
}

// Unfollow 设置当前用户取消关注目标用户。
func (s *Service) Unfollow(ctx context.Context, userID int64, targetUserID int64, idempotencyKey string) (*FollowResult, error) {
	return s.setFollow(ctx, userID, targetUserID, false, idempotencyKey)
}

// ListFollowing 查询当前用户的关注列表。
func (s *Service) ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (*ListResult, error) {
	if userID <= 0 {
		return nil, domainrelation.ErrInvalidUserID
	}
	parsedCursor, err := parseListCursor(cursor)
	if err != nil {
		return nil, err
	}
	limit = normalizeLimit(limit)

	items, err := s.repo.ListFollowing(ctx, userID, parsedCursor, limit+1)
	if err != nil {
		return nil, ErrLoadRelationFailed
	}
	return listResult(items, limit), nil
}

// ListFollowers 查询当前用户的粉丝列表。
func (s *Service) ListFollowers(ctx context.Context, userID int64, cursor string, limit int) (*ListResult, error) {
	if userID <= 0 {
		return nil, domainrelation.ErrInvalidUserID
	}
	parsedCursor, err := parseListCursor(cursor)
	if err != nil {
		return nil, err
	}
	limit = normalizeLimit(limit)

	items, err := s.repo.ListFollowers(ctx, userID, parsedCursor, limit+1)
	if err != nil {
		return nil, ErrLoadRelationFailed
	}
	return listResult(items, limit), nil
}

// setFollow 统一处理关注和取关，active 表示目标关系状态。
func (s *Service) setFollow(ctx context.Context, userID int64, targetUserID int64, active bool, idempotencyKey string) (*FollowResult, error) {
	if _, err := domainrelation.NewFollow(userID, targetUserID, idempotencyKey); err != nil {
		return nil, err
	}

	follow, userStat, targetStat, err := s.repo.SetFollow(ctx, userID, targetUserID, active, idempotencyKey)
	if err != nil {
		if errors.Is(err, domainrelation.ErrTargetUserNotFound) {
			return nil, domainrelation.ErrTargetUserNotFound
		}
		return nil, ErrUpdateRelationFailed
	}
	if active && follow.Active() {
		if err := s.backfillFollowFeed(ctx, userID, targetUserID); err != nil {
			return nil, ErrBackfillFollowFeedFailed
		}
		s.notifyFollow(ctx, userID, targetUserID)
	}

	return &FollowResult{
		UserID:         follow.UserID,
		TargetUserID:   follow.TargetUserID,
		Status:         follow.Status,
		Following:      follow.Active(),
		FollowingCount: userStat.FollowingCount,
		FollowerCount:  targetStat.FollowerCount,
	}, nil
}

func (s *Service) notifyFollow(ctx context.Context, userID int64, targetUserID int64) {
	if s.messageWriter == nil {
		return
	}
	eventID := fmt.Sprintf("relation:follow:%d:%d", targetUserID, userID)
	actor, _ := s.repo.GetUserProfile(ctx, userID)
	if writer, ok := s.messageWriter.(ActorMessageWriter); ok {
		actorNickname := ""
		actorAvatarURL := ""
		if actor != nil {
			actorNickname = actor.Nickname
			actorAvatarURL = actor.AvatarURL
		}
		_, _ = writer.CreateFromActorEvent(
			ctx,
			targetUserID,
			domainmessage.TypeFollow,
			"新增关注",
			"关注了你",
			eventID,
			eventID,
			userID,
			actorNickname,
			actorAvatarURL,
		)
		return
	}
	actorName := fmt.Sprintf("用户 %d", userID)
	if actor != nil && actor.Nickname != "" {
		actorName = actor.Nickname
	}
	_, _ = s.messageWriter.CreateFromEvent(
		ctx,
		targetUserID,
		domainmessage.TypeFollow,
		"新增关注",
		fmt.Sprintf("%s 关注了你", actorName),
		eventID,
		eventID,
	)
}

func (s *Service) backfillFollowFeed(ctx context.Context, userID int64, targetUserID int64) error {
	if s.backfiller == nil {
		return nil
	}
	followerCount, err := s.backfiller.CountFollowers(ctx, targetUserID)
	if err != nil {
		return err
	}
	if followerCount >= domainfeed.BigCreatorFollowerThreshold {
		return nil
	}

	items, err := s.backfiller.ListAuthorRecentVideos(ctx, targetUserID, defaultBackfillVideoLimit)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		if err := s.backfiller.AddInboxItems(ctx, targetUserID, []int64{userID}, item, defaultBackfillInboxMaxLen); err != nil {
			return err
		}
	}
	return nil
}

func listResult(items []*domainrelation.UserItem, limit int) *ListResult {
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor := ""
	if len(items) > 0 {
		last := items[len(items)-1]
		nextCursor = encodeListCursor(&domainrelation.ListCursor{
			FollowedAt: last.FollowedAt,
			UserID:     last.UserID,
		})
	}

	return &ListResult{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return defaultListLimit
	}
	if limit > domainrelation.MaxLimit {
		return domainrelation.MaxLimit
	}
	return limit
}

func parseListCursor(raw string) (*domainrelation.ListCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	content, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		content, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, domainrelation.ErrInvalidCursor
		}
	}

	var payload listCursorPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, domainrelation.ErrInvalidCursor
	}

	followedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.FollowedAt))
	if err != nil || payload.UserID <= 0 {
		return nil, domainrelation.ErrInvalidCursor
	}

	return &domainrelation.ListCursor{
		FollowedAt: followedAt,
		UserID:     payload.UserID,
	}, nil
}

func encodeListCursor(cursor *domainrelation.ListCursor) string {
	if cursor == nil || cursor.UserID <= 0 || cursor.FollowedAt.IsZero() {
		return ""
	}

	content, err := json.Marshal(listCursorPayload{
		FollowedAt: cursor.FollowedAt.UTC().Format(time.RFC3339Nano),
		UserID:     cursor.UserID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(content)
}
