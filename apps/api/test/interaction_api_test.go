package test

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	applicationinteraction "GCFeed/internal/application/interaction"
	domainaccount "GCFeed/internal/domain/account"
	domaininteraction "GCFeed/internal/domain/interaction"
	infrajwt "GCFeed/internal/infra/jwt"
	interfaceshttpinteraction "GCFeed/internal/interfaces/http/interaction"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"

	"github.com/gin-gonic/gin"
)

type interactionActionAPIResponse struct {
	VideoID       int64  `json:"video_id"`
	ActionType    string `json:"action_type"`
	Active        bool   `json:"active"`
	LikeCount     int    `json:"like_count"`
	FavoriteCount int    `json:"favorite_count"`
}

type interactionCommentAPIResponse struct {
	ID            int64     `json:"id"`
	VideoID       int64     `json:"video_id"`
	UserID        int64     `json:"user_id"`
	UserNickname  string    `json:"user_nickname"`
	UserAvatarURL string    `json:"user_avatar_url"`
	Content       string    `json:"content"`
	CreatedAt     time.Time `json:"created_at"`
	CommentCount  int       `json:"comment_count"`
}

type interactionCommentListAPIResponse struct {
	Items      []interactionCommentAPIResponse `json:"items"`
	NextCursor string                          `json:"next_cursor"`
	HasMore    bool                            `json:"has_more"`
}

type interactionDeleteCommentAPIResponse struct {
	CommentID    int64 `json:"comment_id"`
	Status       int   `json:"status"`
	CommentCount int   `json:"comment_count"`
}

type memoryInteractionVideo struct {
	ID       int64
	AuthorID int64
	Status   int
}

type memoryInteractionStat struct {
	LikeCount     int
	CommentCount  int
	FavoriteCount int
}

// memoryInteractionRepo 是互动测试用内存仓储，模拟点赞、收藏、评论和计数。
type memoryInteractionRepo struct {
	mu            sync.Mutex
	nextActionID  int64
	nextCommentID int64
	videos        map[int64]memoryInteractionVideo
	stats         map[int64]memoryInteractionStat
	actions       map[string]*domaininteraction.Action
	comments      map[int64]*domaininteraction.Comment
	commentIdem   map[string]int64
}

type memoryHotScoreRecorder struct {
	mu     sync.Mutex
	scores map[int64]int
	events []int
}

func newMemoryInteractionRepo() *memoryInteractionRepo {
	return &memoryInteractionRepo{
		nextActionID:  1,
		nextCommentID: 1,
		videos: map[int64]memoryInteractionVideo{
			1001: {ID: 1001, AuthorID: 42, Status: 2},
			1002: {ID: 1002, AuthorID: 77, Status: 2},
		},
		stats:       map[int64]memoryInteractionStat{1001: {}, 1002: {}},
		actions:     map[string]*domaininteraction.Action{},
		comments:    map[int64]*domaininteraction.Comment{},
		commentIdem: map[string]int64{},
	}
}

// SetAction 模拟点赞/收藏状态变更，并维护对应计数。
func (r *memoryInteractionRepo) SetAction(ctx context.Context, userID int64, videoID int64, actionType string, active bool, idempotencyKey string) (*domaininteraction.Action, int, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.videoPublished(videoID) {
		return nil, 0, 0, domaininteraction.ErrVideoNotFound
	}

	key := memoryInteractionActionKey(userID, videoID, actionType)
	action, exists := r.actions[key]
	if exists && idempotencyKey != "" && action.IdempotencyKey == strings.TrimSpace(idempotencyKey) {
		// 幂等键命中时直接返回当前状态和计数，模拟真实仓储重放逻辑。
		return cloneInteractionAction(action), r.actionCount(videoID, actionType), 0, nil
	}

	nextStatus := domaininteraction.ActionStatusCanceled
	if active {
		nextStatus = domaininteraction.ActionStatusActive
	}

	delta := 0
	if !exists {
		action = &domaininteraction.Action{
			ID:             r.nextActionID,
			UserID:         userID,
			VideoID:        videoID,
			ActionType:     actionType,
			Status:         nextStatus,
			IdempotencyKey: strings.TrimSpace(idempotencyKey),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		r.nextActionID++
		r.actions[key] = action
		if active {
			delta = 1
			r.addActionCount(videoID, actionType, 1)
		}
		return cloneInteractionAction(action), r.actionCount(videoID, actionType), delta, nil
	}

	if action.Status != nextStatus {
		if active {
			delta = 1
			r.addActionCount(videoID, actionType, 1)
		} else {
			delta = -1
			r.addActionCount(videoID, actionType, -1)
		}
	}
	action.Status = nextStatus
	action.IdempotencyKey = strings.TrimSpace(idempotencyKey)
	action.UpdatedAt = time.Now()
	return cloneInteractionAction(action), r.actionCount(videoID, actionType), delta, nil
}

// CreateComment 模拟评论创建，并维护视频评论数和评论幂等索引。
func (r *memoryInteractionRepo) CreateComment(ctx context.Context, comment *domaininteraction.Comment) (*domaininteraction.Comment, int, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.videoPublished(comment.VideoID) {
		return nil, 0, 0, domaininteraction.ErrVideoNotFound
	}
	if comment.IdempotencyKey != "" {
		key := memoryInteractionCommentIdemKey(comment.UserID, comment.IdempotencyKey)
		if id, exists := r.commentIdem[key]; exists {
			existing := r.comments[id]
			return cloneInteractionComment(existing), r.stats[existing.VideoID].CommentCount, 0, nil
		}
	}

	now := time.Now()
	comment.ID = r.nextCommentID
	r.nextCommentID++
	comment.UserNickname = memoryInteractionNickname(comment.UserID)
	comment.UserAvatarURL = memoryInteractionAvatar(comment.UserID)
	comment.CreatedAt = now
	comment.UpdatedAt = now
	r.comments[comment.ID] = cloneInteractionComment(comment)
	if comment.IdempotencyKey != "" {
		r.commentIdem[memoryInteractionCommentIdemKey(comment.UserID, comment.IdempotencyKey)] = comment.ID
	}
	stat := r.stats[comment.VideoID]
	stat.CommentCount++
	r.stats[comment.VideoID] = stat
	return cloneInteractionComment(comment), stat.CommentCount, 1, nil
}

// FindCommentByUserAndIdempotencyKey 模拟评论创建接口的幂等查询。
func (r *memoryInteractionRepo) FindCommentByUserAndIdempotencyKey(ctx context.Context, userID int64, idempotencyKey string) (*domaininteraction.Comment, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	id, exists := r.commentIdem[memoryInteractionCommentIdemKey(userID, idempotencyKey)]
	if !exists {
		return nil, 0, domaininteraction.ErrCommentNotFound
	}
	comment := r.comments[id]
	return cloneInteractionComment(comment), r.stats[comment.VideoID].CommentCount, nil
}

// ListComments 模拟评论列表游标分页，排序规则与真实仓储一致。
func (r *memoryInteractionRepo) ListComments(ctx context.Context, videoID int64, cursor *domaininteraction.CommentCursor, limit int) ([]*domaininteraction.Comment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	comments := make([]*domaininteraction.Comment, 0)
	for _, comment := range r.comments {
		if comment.VideoID != videoID || comment.Status != domaininteraction.CommentStatusNormal {
			continue
		}
		if cursor != nil && !comment.CreatedAt.Before(cursor.CreatedAt) && !(comment.CreatedAt.Equal(cursor.CreatedAt) && comment.ID < cursor.CommentID) {
			continue
		}
		comments = append(comments, cloneInteractionComment(comment))
	}
	sort.Slice(comments, func(i, j int) bool {
		if comments[i].CreatedAt.Equal(comments[j].CreatedAt) {
			return comments[i].ID > comments[j].ID
		}
		return comments[i].CreatedAt.After(comments[j].CreatedAt)
	})
	if limit > len(comments) {
		limit = len(comments)
	}
	return comments[:limit], nil
}

// DeleteComment 模拟评论软删除，以及评论作者、视频作者、管理员三种删除权限。
func (r *memoryInteractionRepo) DeleteComment(ctx context.Context, commentID int64, userID int64, role string) (*domaininteraction.Comment, int, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	comment, exists := r.comments[commentID]
	if !exists {
		return nil, 0, 0, domaininteraction.ErrCommentNotFound
	}
	video := r.videos[comment.VideoID]
	if comment.UserID != userID && video.AuthorID != userID && role != domainaccount.RoleAdmin {
		return nil, 0, 0, domaininteraction.ErrCommentPermissionDenied
	}
	delta := 0
	if comment.Status != domaininteraction.CommentStatusDeleted {
		comment.Status = domaininteraction.CommentStatusDeleted
		comment.UpdatedAt = time.Now()
		delta = -1
		stat := r.stats[comment.VideoID]
		if stat.CommentCount > 0 {
			stat.CommentCount--
		}
		r.stats[comment.VideoID] = stat
	}
	return cloneInteractionComment(comment), r.stats[comment.VideoID].CommentCount, delta, nil
}

// TestInteractionActionFlow 覆盖点赞、幂等重放、取消点赞和收藏。
func TestInteractionActionFlow(t *testing.T) {
	router, jwtManager := newInteractionRouter(t)
	token := signTestToken(t, jwtManager, 42)

	likeResponse := performVideoJSONRequest(router, http.MethodPut, "/api/videos/1001/like", "", token, "like-1")
	requireStatus(t, likeResponse, http.StatusOK)

	var liked interactionActionAPIResponse
	decodeJSON(t, likeResponse, &liked)
	if liked.ActionType != domaininteraction.ActionTypeLike || !liked.Active || liked.LikeCount != 1 {
		t.Fatalf("unexpected like response: %+v", liked)
	}

	replayResponse := performVideoJSONRequest(router, http.MethodPut, "/api/videos/1001/like", "", token, "like-1")
	requireStatus(t, replayResponse, http.StatusOK)
	var replayed interactionActionAPIResponse
	decodeJSON(t, replayResponse, &replayed)
	if !replayed.Active || replayed.LikeCount != 1 {
		t.Fatalf("unexpected replay response: %+v", replayed)
	}

	unlikeResponse := performVideoJSONRequest(router, http.MethodDelete, "/api/videos/1001/like", "", token, "like-2")
	requireStatus(t, unlikeResponse, http.StatusOK)
	var unliked interactionActionAPIResponse
	decodeJSON(t, unlikeResponse, &unliked)
	if unliked.Active || unliked.LikeCount != 0 {
		t.Fatalf("unexpected unlike response: %+v", unliked)
	}

	favoriteResponse := performVideoJSONRequest(router, http.MethodPut, "/api/videos/1001/favorite", "", token, "favorite-1")
	requireStatus(t, favoriteResponse, http.StatusOK)
	var favorited interactionActionAPIResponse
	decodeJSON(t, favoriteResponse, &favorited)
	if favorited.ActionType != domaininteraction.ActionTypeFavorite || !favorited.Active || favorited.FavoriteCount != 1 {
		t.Fatalf("unexpected favorite response: %+v", favorited)
	}
}

// TestInteractionCommentFlow 覆盖创建评论、幂等重放、列表、权限删除和重复删除。
func TestInteractionCommentFlow(t *testing.T) {
	router, jwtManager := newInteractionRouter(t)
	authorToken := signTestToken(t, jwtManager, 42)
	commenterToken := signTestToken(t, jwtManager, 77)
	otherToken := signTestToken(t, jwtManager, 99)

	createResponse := performVideoJSONRequest(router, http.MethodPost, "/api/videos/1001/comments", `{"content":" first comment "}`, commenterToken, "comment-1")
	requireStatus(t, createResponse, http.StatusCreated)

	var created interactionCommentAPIResponse
	decodeJSON(t, createResponse, &created)
	if created.ID == 0 || created.UserID != 77 || created.Content != "first comment" || created.CommentCount != 1 {
		t.Fatalf("unexpected comment response: %+v", created)
	}

	replayResponse := performVideoJSONRequest(router, http.MethodPost, "/api/videos/1001/comments", `{"content":"changed"}`, commenterToken, "comment-1")
	requireStatus(t, replayResponse, http.StatusCreated)
	var replayed interactionCommentAPIResponse
	decodeJSON(t, replayResponse, &replayed)
	if replayed.ID != created.ID || replayed.Content != created.Content || replayed.CommentCount != 1 {
		t.Fatalf("unexpected replay comment response: %+v", replayed)
	}

	listResponse := performJSONRequest(router, http.MethodGet, "/api/videos/1001/comments?limit=10", "", "")
	requireStatus(t, listResponse, http.StatusOK)
	var list interactionCommentListAPIResponse
	decodeJSON(t, listResponse, &list)
	if len(list.Items) != 1 || list.Items[0].ID != created.ID || list.Items[0].UserNickname != "user-77" {
		t.Fatalf("unexpected comment list response: %+v", list)
	}

	forbiddenDelete := performJSONRequest(router, http.MethodDelete, "/api/comments/1", "", otherToken)
	requireStatus(t, forbiddenDelete, http.StatusForbidden)

	authorDelete := performJSONRequest(router, http.MethodDelete, "/api/comments/1", "", authorToken)
	requireStatus(t, authorDelete, http.StatusOK)
	var deleted interactionDeleteCommentAPIResponse
	decodeJSON(t, authorDelete, &deleted)
	if deleted.CommentID != created.ID || deleted.Status != domaininteraction.CommentStatusDeleted || deleted.CommentCount != 0 {
		t.Fatalf("unexpected delete response: %+v", deleted)
	}

	repeatDelete := performJSONRequest(router, http.MethodDelete, "/api/comments/1", "", authorToken)
	requireStatus(t, repeatDelete, http.StatusOK)
	var repeat interactionDeleteCommentAPIResponse
	decodeJSON(t, repeatDelete, &repeat)
	if repeat.CommentCount != 0 {
		t.Fatalf("unexpected repeat delete response: %+v", repeat)
	}
}

// TestInteractionValidation 覆盖互动接口的登录态、参数和资源校验。
func TestInteractionValidation(t *testing.T) {
	router, jwtManager := newInteractionRouter(t)
	token := signTestToken(t, jwtManager, 42)

	unauthorizedLike := performJSONRequest(router, http.MethodPut, "/api/videos/1001/like", "", "")
	requireStatus(t, unauthorizedLike, http.StatusUnauthorized)

	badLike := performJSONRequest(router, http.MethodPut, "/api/videos/0/like", "", token)
	requireStatus(t, badLike, http.StatusBadRequest)

	missingVideo := performJSONRequest(router, http.MethodPut, "/api/videos/404/like", "", token)
	requireStatus(t, missingVideo, http.StatusNotFound)

	emptyComment := performJSONRequest(router, http.MethodPost, "/api/videos/1001/comments", `{"content":"   "}`, token)
	requireStatus(t, emptyComment, http.StatusBadRequest)

	badList := performJSONRequest(router, http.MethodGet, "/api/videos/1001/comments?limit=0", "", "")
	requireStatus(t, badList, http.StatusBadRequest)
}

// TestInteractionHotScoreRecorder 覆盖真实互动变化写入热榜增量。
func TestInteractionHotScoreRecorder(t *testing.T) {
	repo := newMemoryInteractionRepo()
	recorder := newMemoryHotScoreRecorder()
	service := applicationinteraction.New(repo, applicationinteraction.WithHotScoreRecorder(recorder))

	if _, err := service.Like(context.Background(), 42, 1001, "like-1"); err != nil {
		t.Fatalf("like: %v", err)
	}
	if _, err := service.Like(context.Background(), 42, 1001, "like-1"); err != nil {
		t.Fatalf("like replay: %v", err)
	}
	if _, err := service.Favorite(context.Background(), 42, 1001, "favorite-1"); err != nil {
		t.Fatalf("favorite: %v", err)
	}
	if _, err := service.Unlike(context.Background(), 42, 1001, "like-2"); err != nil {
		t.Fatalf("unlike: %v", err)
	}
	created, err := service.CreateComment(context.Background(), 77, 1001, "hot comment", "comment-1")
	if err != nil {
		t.Fatalf("create comment: %v", err)
	}
	if _, err := service.CreateComment(context.Background(), 77, 1001, "hot comment replay", "comment-1"); err != nil {
		t.Fatalf("comment replay: %v", err)
	}
	if _, err := service.DeleteComment(context.Background(), created.Comment.ID, 77, domainaccount.RoleUser); err != nil {
		t.Fatalf("delete comment: %v", err)
	}

	if recorder.Score(1001) != 4 {
		t.Fatalf("unexpected hot score: %d", recorder.Score(1001))
	}
	if recorder.EventCount() != 5 {
		t.Fatalf("unexpected hot event count: %d", recorder.EventCount())
	}
}

// newInteractionRouter 只装配互动相关 RESTful 路由。
func newInteractionRouter(t *testing.T) (*gin.Engine, *infrajwt.Manager) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	jwtManager, err := infrajwt.NewManager("test-secret", "15m")
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}

	repo := newMemoryInteractionRepo()
	service := applicationinteraction.New(repo)
	handler := interfaceshttpinteraction.New(service)
	authMiddleware := interfaceshttpmiddleware.NewJWTAuth(jwtManager)

	api := router.Group("/api")
	videos := api.Group("/videos")
	videos.PUT("/:videoId/like", authMiddleware, handler.Like)
	videos.DELETE("/:videoId/like", authMiddleware, handler.Unlike)
	videos.PUT("/:videoId/favorite", authMiddleware, handler.Favorite)
	videos.DELETE("/:videoId/favorite", authMiddleware, handler.Unfavorite)
	videos.POST("/:videoId/comments", authMiddleware, handler.CreateComment)
	videos.GET("/:videoId/comments", handler.ListComments)
	api.DELETE("/comments/:commentId", authMiddleware, handler.DeleteComment)

	return router, jwtManager
}

// videoPublished 模拟互动前校验视频是否可互动。
func (r *memoryInteractionRepo) videoPublished(videoID int64) bool {
	video, exists := r.videos[videoID]
	return exists && video.Status == 2
}

// addActionCount 根据行为类型增加或减少点赞/收藏数。
func (r *memoryInteractionRepo) addActionCount(videoID int64, actionType string, delta int) {
	stat := r.stats[videoID]
	if actionType == domaininteraction.ActionTypeLike {
		stat.LikeCount = clampMemoryCount(stat.LikeCount + delta)
	} else {
		stat.FavoriteCount = clampMemoryCount(stat.FavoriteCount + delta)
	}
	r.stats[videoID] = stat
}

// actionCount 根据行为类型返回当前点赞数或收藏数。
func (r *memoryInteractionRepo) actionCount(videoID int64, actionType string) int {
	stat := r.stats[videoID]
	if actionType == domaininteraction.ActionTypeLike {
		return stat.LikeCount
	}
	return stat.FavoriteCount
}

// memoryInteractionActionKey 模拟 user_id + video_id + action_type 唯一索引。
func memoryInteractionActionKey(userID int64, videoID int64, actionType string) string {
	return strings.Join([]string{int64String(userID), int64String(videoID), actionType}, ":")
}

// memoryInteractionCommentIdemKey 模拟 user_id + idempotency_key 唯一索引。
func memoryInteractionCommentIdemKey(userID int64, idempotencyKey string) string {
	return int64String(userID) + ":" + strings.TrimSpace(idempotencyKey)
}

// int64String 统一测试里 int64 ID 的字符串转换。
func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}

// memoryInteractionNickname 为测试评论补齐用户昵称。
func memoryInteractionNickname(userID int64) string {
	if userID == 77 {
		return "user-77"
	}
	return "user"
}

// memoryInteractionAvatar 为测试评论补齐用户头像。
func memoryInteractionAvatar(userID int64) string {
	if userID == 77 {
		return "https://example.com/avatar-77.jpg"
	}
	return ""
}

// cloneInteractionAction 返回互动行为副本，隔离仓储内部状态。
func cloneInteractionAction(action *domaininteraction.Action) *domaininteraction.Action {
	cloned := *action
	return &cloned
}

// cloneInteractionComment 返回评论副本，隔离仓储内部状态。
func cloneInteractionComment(comment *domaininteraction.Comment) *domaininteraction.Comment {
	cloned := *comment
	return &cloned
}

// clampMemoryCount 防止测试仓储中的计数被减成负数。
func clampMemoryCount(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func newMemoryHotScoreRecorder() *memoryHotScoreRecorder {
	return &memoryHotScoreRecorder{
		scores: map[int64]int{},
		events: []int{},
	}
}

func (r *memoryHotScoreRecorder) AddHotScore(ctx context.Context, videoID int64, scoreDelta int, at time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.scores[videoID] += scoreDelta
	r.events = append(r.events, scoreDelta)
	return nil
}

func (r *memoryHotScoreRecorder) Score(videoID int64) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.scores[videoID]
}

func (r *memoryHotScoreRecorder) EventCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.events)
}

var _ domaininteraction.Repository = (*memoryInteractionRepo)(nil)
