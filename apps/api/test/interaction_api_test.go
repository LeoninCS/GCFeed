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

func (r *memoryInteractionRepo) SetAction(ctx context.Context, userID int64, videoID int64, actionType string, active bool, idempotencyKey string) (*domaininteraction.Action, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.videoPublished(videoID) {
		return nil, 0, domaininteraction.ErrVideoNotFound
	}

	key := memoryInteractionActionKey(userID, videoID, actionType)
	action, exists := r.actions[key]
	if exists && idempotencyKey != "" && action.IdempotencyKey == strings.TrimSpace(idempotencyKey) {
		return cloneInteractionAction(action), r.actionCount(videoID, actionType), nil
	}

	nextStatus := domaininteraction.ActionStatusCanceled
	if active {
		nextStatus = domaininteraction.ActionStatusActive
	}

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
			r.addActionCount(videoID, actionType, 1)
		}
		return cloneInteractionAction(action), r.actionCount(videoID, actionType), nil
	}

	if action.Status != nextStatus {
		if active {
			r.addActionCount(videoID, actionType, 1)
		} else {
			r.addActionCount(videoID, actionType, -1)
		}
	}
	action.Status = nextStatus
	action.IdempotencyKey = strings.TrimSpace(idempotencyKey)
	action.UpdatedAt = time.Now()
	return cloneInteractionAction(action), r.actionCount(videoID, actionType), nil
}

func (r *memoryInteractionRepo) CreateComment(ctx context.Context, comment *domaininteraction.Comment) (*domaininteraction.Comment, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.videoPublished(comment.VideoID) {
		return nil, 0, domaininteraction.ErrVideoNotFound
	}
	if comment.IdempotencyKey != "" {
		key := memoryInteractionCommentIdemKey(comment.UserID, comment.IdempotencyKey)
		if id, exists := r.commentIdem[key]; exists {
			existing := r.comments[id]
			return cloneInteractionComment(existing), r.stats[existing.VideoID].CommentCount, nil
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
	return cloneInteractionComment(comment), stat.CommentCount, nil
}

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

func (r *memoryInteractionRepo) DeleteComment(ctx context.Context, commentID int64, userID int64, role string) (*domaininteraction.Comment, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	comment, exists := r.comments[commentID]
	if !exists {
		return nil, 0, domaininteraction.ErrCommentNotFound
	}
	video := r.videos[comment.VideoID]
	if comment.UserID != userID && video.AuthorID != userID && role != domainaccount.RoleAdmin {
		return nil, 0, domaininteraction.ErrCommentPermissionDenied
	}
	if comment.Status != domaininteraction.CommentStatusDeleted {
		comment.Status = domaininteraction.CommentStatusDeleted
		comment.UpdatedAt = time.Now()
		stat := r.stats[comment.VideoID]
		if stat.CommentCount > 0 {
			stat.CommentCount--
		}
		r.stats[comment.VideoID] = stat
	}
	return cloneInteractionComment(comment), r.stats[comment.VideoID].CommentCount, nil
}

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

func newInteractionRouter(t *testing.T) (*gin.Engine, *infrajwt.Manager) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	jwtManager, err := infrajwt.NewManager("test-secret", "15m")
	if err != nil {
		t.Fatalf("new jwt manager: %v", err)
	}

	repo := newMemoryInteractionRepo()
	service := applicationinteraction.NewService(repo)
	handler := interfaceshttpinteraction.NewHandler(service)
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

func (r *memoryInteractionRepo) videoPublished(videoID int64) bool {
	video, exists := r.videos[videoID]
	return exists && video.Status == 2
}

func (r *memoryInteractionRepo) addActionCount(videoID int64, actionType string, delta int) {
	stat := r.stats[videoID]
	if actionType == domaininteraction.ActionTypeLike {
		stat.LikeCount = clampMemoryCount(stat.LikeCount + delta)
	} else {
		stat.FavoriteCount = clampMemoryCount(stat.FavoriteCount + delta)
	}
	r.stats[videoID] = stat
}

func (r *memoryInteractionRepo) actionCount(videoID int64, actionType string) int {
	stat := r.stats[videoID]
	if actionType == domaininteraction.ActionTypeLike {
		return stat.LikeCount
	}
	return stat.FavoriteCount
}

func memoryInteractionActionKey(userID int64, videoID int64, actionType string) string {
	return strings.Join([]string{int64String(userID), int64String(videoID), actionType}, ":")
}

func memoryInteractionCommentIdemKey(userID int64, idempotencyKey string) string {
	return int64String(userID) + ":" + strings.TrimSpace(idempotencyKey)
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}

func memoryInteractionNickname(userID int64) string {
	if userID == 77 {
		return "user-77"
	}
	return "user"
}

func memoryInteractionAvatar(userID int64) string {
	if userID == 77 {
		return "https://example.com/avatar-77.jpg"
	}
	return ""
}

func cloneInteractionAction(action *domaininteraction.Action) *domaininteraction.Action {
	cloned := *action
	return &cloned
}

func cloneInteractionComment(comment *domaininteraction.Comment) *domaininteraction.Comment {
	cloned := *comment
	return &cloned
}

func clampMemoryCount(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

var _ domaininteraction.Repository = (*memoryInteractionRepo)(nil)
