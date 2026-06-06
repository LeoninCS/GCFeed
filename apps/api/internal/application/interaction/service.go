package applicationinteraction

import (
	domaininteraction "GCFeed/internal/domain/interaction"
	domainmessage "GCFeed/internal/domain/message"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const defaultCommentLimit = 20
const hotScoreLikeWeight = 3
const hotScoreFavoriteWeight = 4
const hotScoreCommentWeight = 5

var ErrLoadInteractionFailed = errors.New("failed to load interaction")
var ErrSaveInteractionFailed = errors.New("failed to save interaction")
var ErrUpdateInteractionFailed = errors.New("failed to update interaction")

type Service struct {
	repo             domaininteraction.Repository
	hotScoreRecorder HotScoreRecorder
	statCache        StatCache
	actionStateStore ActionStateStore
	actionPublisher  ActionEventPublisher
	messageWriter    MessageWriter
}

// HotScoreRecorder 把互动变化投递到热榜分钟桶。
type HotScoreRecorder interface {
	AddHotScore(ctx context.Context, videoID int64, scoreDelta int, at time.Time) error
}

// StatCache 同步 Feed 展示所需的视频互动计数缓存。
type StatCache interface {
	SetVideoStat(ctx context.Context, stat *domaininteraction.VideoStat) error
}

type Option func(*Service)

type ActionStateResult struct {
	VideoID        int64
	ActionType     string
	Active         bool
	LikeCount      int
	FavoriteCount  int
	Delta          int
	IdempotencyKey string
}

// ActionStateStore 保存点赞收藏的快速状态和计数。
type ActionStateStore interface {
	SetActionState(ctx context.Context, userID int64, videoID int64, actionType string, active bool, idempotencyKey string, initialStat *domaininteraction.VideoStat) (*ActionStateResult, error)
}

// ActionEventPublisher 投递点赞收藏变更事件。
type ActionEventPublisher interface {
	PublishActionChanged(ctx context.Context, event *ActionChangedEvent) error
}

// MessageWriter 写入互动触发的站内消息。
type MessageWriter interface {
	CreateFromEvent(ctx context.Context, userID int64, messageType string, title string, content string, eventID string, idempotencyKey string) (any, error)
}

// ActorMessageWriter 可在消息里携带触发互动的用户资料。
type ActorMessageWriter interface {
	CreateFromActorEvent(ctx context.Context, userID int64, messageType string, title string, content string, eventID string, idempotencyKey string, actorID int64, actorNickname string, actorAvatarURL string) (any, error)
}

type ActionChangedEvent struct {
	EventID        string    `json:"event_id"`
	UserID         int64     `json:"user_id"`
	VideoID        int64     `json:"video_id"`
	ActionType     string    `json:"action_type"`
	Active         bool      `json:"active"`
	IdempotencyKey string    `json:"idempotency_key"`
	OccurredAt     time.Time `json:"occurred_at"`
}

type ActionResult struct {
	VideoID       int64
	ActionType    string
	Active        bool
	LikeCount     int
	FavoriteCount int
}

type CreateCommentResult struct {
	Comment      *domaininteraction.Comment
	CommentCount int
}

type DeleteCommentResult struct {
	CommentID    int64
	Status       int
	CommentCount int
}

type CommentListResult struct {
	Items      []*domaininteraction.Comment
	NextCursor string
	HasMore    bool
}

type commentCursorPayload struct {
	CreatedAt string `json:"created_at"`
	CommentID int64  `json:"comment_id"`
}

func New(repo domaininteraction.Repository, options ...Option) *Service {
	service := &Service{repo: repo}
	for _, option := range options {
		option(service)
	}
	return service
}

// WithHotScoreRecorder 为互动服务启用热榜增量写入。
func WithHotScoreRecorder(recorder HotScoreRecorder) Option {
	return func(s *Service) {
		s.hotScoreRecorder = recorder
	}
}

// WithStatCache 为评论写入后的 Feed 计数展示启用缓存同步。
func WithStatCache(cache StatCache) Option {
	return func(s *Service) {
		s.statCache = cache
	}
}

// WithAsyncActionPipeline 为点赞收藏启用 Redis 快速写和 MQ 异步落库。
func WithAsyncActionPipeline(store ActionStateStore, publisher ActionEventPublisher) Option {
	return func(s *Service) {
		s.actionStateStore = store
		s.actionPublisher = publisher
	}
}

// WithMessageWriter 为点赞和评论成功后的通知写入启用消息中心。
func WithMessageWriter(writer MessageWriter) Option {
	return func(s *Service) {
		s.messageWriter = writer
	}
}

// Like 设置用户对视频的点赞状态为有效。
func (s *Service) Like(ctx context.Context, userID int64, videoID int64, idempotencyKey string) (*ActionResult, error) {
	return s.setAction(ctx, userID, videoID, domaininteraction.ActionTypeLike, true, idempotencyKey)
}

// Unlike 设置用户对视频的点赞状态为取消。
func (s *Service) Unlike(ctx context.Context, userID int64, videoID int64, idempotencyKey string) (*ActionResult, error) {
	return s.setAction(ctx, userID, videoID, domaininteraction.ActionTypeLike, false, idempotencyKey)
}

// Favorite 设置用户对视频的收藏状态为有效。
func (s *Service) Favorite(ctx context.Context, userID int64, videoID int64, idempotencyKey string) (*ActionResult, error) {
	return s.setAction(ctx, userID, videoID, domaininteraction.ActionTypeFavorite, true, idempotencyKey)
}

// Unfavorite 设置用户对视频的收藏状态为取消。
func (s *Service) Unfavorite(ctx context.Context, userID int64, videoID int64, idempotencyKey string) (*ActionResult, error) {
	return s.setAction(ctx, userID, videoID, domaininteraction.ActionTypeFavorite, false, idempotencyKey)
}

// CreateComment 创建评论，并通过幂等键防止客户端重试生成重复评论。
func (s *Service) CreateComment(ctx context.Context, userID int64, videoID int64, content string, idempotencyKey string) (*CreateCommentResult, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if len(idempotencyKey) > domaininteraction.MaxIdempotencyKeyLength {
		return nil, domaininteraction.ErrIdempotencyKeyTooLong
	}

	if idempotencyKey != "" {
		// 幂等键命中时返回已创建的评论，客户端重试可以拿到同一结果。
		comment, count, err := s.repo.FindCommentByUserAndIdempotencyKey(ctx, userID, idempotencyKey)
		if err == nil {
			return &CreateCommentResult{Comment: comment, CommentCount: count}, nil
		}
		if !errors.Is(err, domaininteraction.ErrCommentNotFound) {
			return nil, ErrLoadInteractionFailed
		}
	}

	comment, err := domaininteraction.NewComment(videoID, userID, content, idempotencyKey)
	if err != nil {
		return nil, err
	}

	created, count, delta, err := s.repo.CreateComment(ctx, comment)
	if err != nil {
		if errors.Is(err, domaininteraction.ErrVideoNotFound) {
			return nil, domaininteraction.ErrVideoNotFound
		}
		return nil, ErrSaveInteractionFailed
	}
	s.recordHotScore(ctx, created.VideoID, delta*hotScoreCommentWeight)
	s.syncCommentCount(ctx, created.VideoID, count)
	if delta > 0 {
		s.notifyComment(ctx, created)
	}

	return &CreateCommentResult{Comment: created, CommentCount: count}, nil
}

// ListComments 使用游标分页查询评论，返回下一页游标和 has_more。
func (s *Service) ListComments(ctx context.Context, videoID int64, cursor string, limit int) (*CommentListResult, error) {
	if videoID <= 0 {
		return nil, domaininteraction.ErrInvalidVideoID
	}

	parsedCursor, err := parseCommentCursor(cursor)
	if err != nil {
		return nil, err
	}
	limit = normalizeCommentLimit(limit)

	// 多查 1 条用于判断是否还有下一页，返回给客户端时再裁掉。
	items, err := s.repo.ListComments(ctx, videoID, parsedCursor, limit+1)
	if err != nil {
		return nil, ErrLoadInteractionFailed
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	nextCursor := ""
	if len(items) > 0 {
		nextCursor = encodeCommentCursor(&domaininteraction.CommentCursor{
			CreatedAt: items[len(items)-1].CreatedAt,
			CommentID: items[len(items)-1].ID,
		})
	}

	return &CommentListResult{
		Items:      items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}

// DeleteComment 删除评论并返回删除后的评论状态和视频评论数。
func (s *Service) DeleteComment(ctx context.Context, commentID int64, userID int64, role string) (*DeleteCommentResult, error) {
	if commentID <= 0 {
		return nil, domaininteraction.ErrInvalidCommentID
	}
	if userID <= 0 {
		return nil, domaininteraction.ErrInvalidUserID
	}

	comment, count, delta, err := s.repo.DeleteComment(ctx, commentID, userID, role)
	if err != nil {
		if errors.Is(err, domaininteraction.ErrCommentNotFound) ||
			errors.Is(err, domaininteraction.ErrCommentPermissionDenied) {
			return nil, err
		}
		return nil, ErrUpdateInteractionFailed
	}
	s.recordHotScore(ctx, comment.VideoID, delta*hotScoreCommentWeight)
	s.syncCommentCount(ctx, comment.VideoID, count)

	return &DeleteCommentResult{
		CommentID:    comment.ID,
		Status:       comment.Status,
		CommentCount: count,
	}, nil
}

// setAction 统一处理点赞和收藏状态变更，actionType 区分点赞或收藏，active 表示目标状态。
func (s *Service) setAction(ctx context.Context, userID int64, videoID int64, actionType string, active bool, idempotencyKey string) (*ActionResult, error) {
	if userID <= 0 {
		return nil, domaininteraction.ErrInvalidUserID
	}
	if videoID <= 0 {
		return nil, domaininteraction.ErrInvalidVideoID
	}
	if len(strings.TrimSpace(idempotencyKey)) > domaininteraction.MaxIdempotencyKeyLength {
		return nil, domaininteraction.ErrIdempotencyKeyTooLong
	}

	actionType, err := domaininteraction.NormalizeActionType(actionType)
	if err != nil {
		return nil, err
	}
	if s.actionStateStore != nil && s.actionPublisher != nil {
		return s.setActionAsync(ctx, userID, videoID, actionType, active, idempotencyKey)
	}

	return s.setActionSync(ctx, userID, videoID, actionType, active, idempotencyKey)
}

func (s *Service) setActionAsync(ctx context.Context, userID int64, videoID int64, actionType string, active bool, idempotencyKey string) (*ActionResult, error) {
	initialStat, err := s.repo.GetVideoStat(ctx, videoID)
	if err != nil {
		if errors.Is(err, domaininteraction.ErrVideoNotFound) {
			return nil, domaininteraction.ErrVideoNotFound
		}
		return nil, ErrUpdateInteractionFailed
	}

	state, err := s.actionStateStore.SetActionState(ctx, userID, videoID, actionType, active, idempotencyKey, initialStat)
	if err != nil {
		return nil, ErrUpdateInteractionFailed
	}

	if state.Delta != 0 {
		event := NewActionChangedEvent(userID, videoID, actionType, active, idempotencyKey)
		if err := s.actionPublisher.PublishActionChanged(ctx, event); err != nil {
			return s.setActionSync(ctx, userID, videoID, actionType, active, idempotencyKey)
		}
		s.recordActionHotScore(ctx, state.VideoID, state.ActionType, state.Delta)
		if state.ActionType == domaininteraction.ActionTypeLike && state.Active && state.Delta > 0 {
			s.notifyLike(ctx, &domaininteraction.Action{
				UserID:     userID,
				VideoID:    state.VideoID,
				ActionType: state.ActionType,
				Status:     domaininteraction.ActionStatusActive,
			})
		}
	}

	return &ActionResult{
		VideoID:       state.VideoID,
		ActionType:    state.ActionType,
		Active:        state.Active,
		LikeCount:     state.LikeCount,
		FavoriteCount: state.FavoriteCount,
	}, nil
}

func (s *Service) setActionSync(ctx context.Context, userID int64, videoID int64, actionType string, active bool, idempotencyKey string) (*ActionResult, error) {
	action, count, delta, err := s.repo.SetAction(ctx, userID, videoID, actionType, active, idempotencyKey)
	if err != nil {
		if errors.Is(err, domaininteraction.ErrVideoNotFound) {
			return nil, domaininteraction.ErrVideoNotFound
		}
		return nil, ErrUpdateInteractionFailed
	}

	result := &ActionResult{
		VideoID:    action.VideoID,
		ActionType: action.ActionType,
		Active:     action.Active(),
	}
	if action.ActionType == domaininteraction.ActionTypeLike {
		result.LikeCount = count
	} else {
		result.FavoriteCount = count
	}
	s.recordActionHotScore(ctx, action.VideoID, action.ActionType, delta)
	if action.ActionType == domaininteraction.ActionTypeLike && action.Active() && delta > 0 {
		s.notifyLike(ctx, action)
	}
	return result, nil
}

func (s *Service) recordActionHotScore(ctx context.Context, videoID int64, actionType string, delta int) {
	recordActionHotScore(ctx, s.hotScoreRecorder, videoID, actionType, delta)
}

func (s *Service) recordHotScore(ctx context.Context, videoID int64, scoreDelta int) {
	recordHotScore(ctx, s.hotScoreRecorder, videoID, scoreDelta)
}

func (s *Service) syncCommentCount(ctx context.Context, videoID int64, commentCount int) {
	if s.statCache == nil || videoID <= 0 {
		return
	}
	stat, err := s.repo.GetVideoStat(ctx, videoID)
	if err != nil {
		stat = &domaininteraction.VideoStat{VideoID: videoID}
	}
	stat.CommentCount = commentCount
	_ = s.statCache.SetVideoStat(ctx, stat)
}

func recordActionHotScore(ctx context.Context, recorder HotScoreRecorder, videoID int64, actionType string, delta int) {
	if actionType == domaininteraction.ActionTypeLike {
		recordHotScore(ctx, recorder, videoID, delta*hotScoreLikeWeight)
		return
	}
	recordHotScore(ctx, recorder, videoID, delta*hotScoreFavoriteWeight)
}

func recordHotScore(ctx context.Context, recorder HotScoreRecorder, videoID int64, scoreDelta int) {
	if recorder == nil || scoreDelta == 0 {
		return
	}
	_ = recorder.AddHotScore(ctx, videoID, scoreDelta, time.Now())
}

func NewActionChangedEvent(userID int64, videoID int64, actionType string, active bool, idempotencyKey string) *ActionChangedEvent {
	return &ActionChangedEvent{
		EventID:        newEventID(),
		UserID:         userID,
		VideoID:        videoID,
		ActionType:     actionType,
		Active:         active,
		IdempotencyKey: strings.TrimSpace(idempotencyKey),
		OccurredAt:     time.Now().UTC(),
	}
}

func (s *Service) notifyLike(ctx context.Context, action *domaininteraction.Action) {
	if s.messageWriter == nil || action == nil {
		return
	}
	authorID, err := s.repo.GetVideoAuthorID(ctx, action.VideoID)
	if err != nil || authorID == action.UserID {
		return
	}
	eventID := fmt.Sprintf("interaction:like:%d:%d", action.VideoID, action.UserID)
	s.createInteractionMessage(ctx, authorID, domainmessage.TypeLike, "收到点赞", "点赞了你的视频", eventID, action.UserID)
}

func (s *Service) notifyComment(ctx context.Context, comment *domaininteraction.Comment) {
	if s.messageWriter == nil || comment == nil {
		return
	}
	authorID, err := s.repo.GetVideoAuthorID(ctx, comment.VideoID)
	if err != nil || authorID == comment.UserID {
		return
	}
	eventID := fmt.Sprintf("interaction:comment:%d", comment.ID)
	s.createInteractionMessage(ctx, authorID, domainmessage.TypeComment, "收到评论", comment.Content, eventID, comment.UserID)
}

func (s *Service) createInteractionMessage(ctx context.Context, userID int64, messageType string, title string, content string, eventID string, actorID int64) {
	actor, _ := s.repo.GetUserProfile(ctx, actorID)
	if writer, ok := s.messageWriter.(ActorMessageWriter); ok {
		actorNickname := ""
		actorAvatarURL := ""
		if actor != nil {
			actorNickname = actor.Nickname
			actorAvatarURL = actor.AvatarURL
		}
		_, _ = writer.CreateFromActorEvent(ctx, userID, messageType, title, content, eventID, eventID, actorID, actorNickname, actorAvatarURL)
		return
	}
	actorName := fmt.Sprintf("用户 %d", actorID)
	if actor != nil && actor.Nickname != "" {
		actorName = actor.Nickname
	}
	_, _ = s.messageWriter.CreateFromEvent(ctx, userID, messageType, title, fmt.Sprintf("%s %s", actorName, content), eventID, eventID)
}

func newEventID() string {
	content := make([]byte, 12)
	if _, err := rand.Read(content); err == nil {
		return hex.EncodeToString(content)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// normalizeCommentLimit 统一评论分页默认值和最大值。
func normalizeCommentLimit(limit int) int {
	if limit <= 0 {
		return defaultCommentLimit
	}
	if limit > domaininteraction.MaxLimit {
		return domaininteraction.MaxLimit
	}
	return limit
}

// parseCommentCursor 解析上一页返回的游标，游标内保存最后一条评论的排序字段。
func parseCommentCursor(raw string) (*domaininteraction.CommentCursor, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	content, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		content, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, domaininteraction.ErrInvalidCursor
		}
	}

	var payload commentCursorPayload
	if err := json.Unmarshal(content, &payload); err != nil {
		return nil, domaininteraction.ErrInvalidCursor
	}

	createdAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(payload.CreatedAt))
	if err != nil || payload.CommentID <= 0 {
		return nil, domaininteraction.ErrInvalidCursor
	}

	return &domaininteraction.CommentCursor{
		CreatedAt: createdAt,
		CommentID: payload.CommentID,
	}, nil
}

// encodeCommentCursor 把当前页最后一条评论的排序字段编码成下一页游标。
func encodeCommentCursor(cursor *domaininteraction.CommentCursor) string {
	if cursor == nil || cursor.CommentID <= 0 || cursor.CreatedAt.IsZero() {
		return ""
	}

	content, err := json.Marshal(commentCursorPayload{
		CreatedAt: cursor.CreatedAt.UTC().Format(time.RFC3339Nano),
		CommentID: cursor.CommentID,
	})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(content)
}
