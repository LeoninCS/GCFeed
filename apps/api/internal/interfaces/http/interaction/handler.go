package interfaceshttpinteraction

import (
	applicationinteraction "GCFeed/internal/application/interaction"
	domaininteraction "GCFeed/internal/domain/interaction"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *applicationinteraction.Service
}

func NewHandler(service *applicationinteraction.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Like(c *gin.Context) {
	h.setLike(c, true)
}

func (h *Handler) Unlike(c *gin.Context) {
	h.setLike(c, false)
}

func (h *Handler) Favorite(c *gin.Context) {
	h.setFavorite(c, true)
}

func (h *Handler) Unfavorite(c *gin.Context) {
	h.setFavorite(c, false)
}

func (h *Handler) CreateComment(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	videoID, err := parsePositiveInt64(c.Param("videoId"), domaininteraction.ErrInvalidVideoID)
	if err != nil {
		writeInteractionError(c, err)
		return
	}

	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	result, err := h.service.CreateComment(c.Request.Context(), userID, videoID, req.Content, c.GetHeader("Idempotency-Key"))
	if err != nil {
		writeInteractionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, commentResponseFromResult(result))
}

func (h *Handler) ListComments(c *gin.Context) {
	videoID, err := parsePositiveInt64(c.Param("videoId"), domaininteraction.ErrInvalidVideoID)
	if err != nil {
		writeInteractionError(c, err)
		return
	}

	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		writeInteractionError(c, err)
		return
	}

	result, err := h.service.ListComments(c.Request.Context(), videoID, c.Query("cursor"), limit)
	if err != nil {
		writeInteractionError(c, err)
		return
	}
	c.JSON(http.StatusOK, commentListResponseFromResult(result))
}

func (h *Handler) DeleteComment(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}
	role := roleFromContext(c)

	commentID, err := parsePositiveInt64(c.Param("commentId"), domaininteraction.ErrInvalidCommentID)
	if err != nil {
		writeInteractionError(c, err)
		return
	}

	result, err := h.service.DeleteComment(c.Request.Context(), commentID, userID, role)
	if err != nil {
		writeInteractionError(c, err)
		return
	}
	c.JSON(http.StatusOK, deleteCommentResponse{
		CommentID:    result.CommentID,
		Status:       result.Status,
		CommentCount: result.CommentCount,
	})
}

func (h *Handler) setLike(c *gin.Context, active bool) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	videoID, err := parsePositiveInt64(c.Param("videoId"), domaininteraction.ErrInvalidVideoID)
	if err != nil {
		writeInteractionError(c, err)
		return
	}

	var result *applicationinteraction.ActionResult
	if active {
		result, err = h.service.Like(c.Request.Context(), userID, videoID, c.GetHeader("Idempotency-Key"))
	} else {
		result, err = h.service.Unlike(c.Request.Context(), userID, videoID, c.GetHeader("Idempotency-Key"))
	}
	if err != nil {
		writeInteractionError(c, err)
		return
	}
	c.JSON(http.StatusOK, actionResponseFromResult(result))
}

func (h *Handler) setFavorite(c *gin.Context, active bool) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	videoID, err := parsePositiveInt64(c.Param("videoId"), domaininteraction.ErrInvalidVideoID)
	if err != nil {
		writeInteractionError(c, err)
		return
	}

	var result *applicationinteraction.ActionResult
	if active {
		result, err = h.service.Favorite(c.Request.Context(), userID, videoID, c.GetHeader("Idempotency-Key"))
	} else {
		result, err = h.service.Unfavorite(c.Request.Context(), userID, videoID, c.GetHeader("Idempotency-Key"))
	}
	if err != nil {
		writeInteractionError(c, err)
		return
	}
	c.JSON(http.StatusOK, actionResponseFromResult(result))
}

func userIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(interfaceshttpmiddleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok && userID > 0
}

func roleFromContext(c *gin.Context) string {
	value, exists := c.Get(interfaceshttpmiddleware.ContextRoleKey)
	if !exists {
		return ""
	}
	role, _ := value.(string)
	return role
}

func parsePositiveInt64(raw string, fallback error) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, fallback
	}
	return value, nil
}

func parseLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, domaininteraction.ErrInvalidLimit
	}
	return limit, nil
}

func actionResponseFromResult(result *applicationinteraction.ActionResult) actionResponse {
	return actionResponse{
		VideoID:       result.VideoID,
		ActionType:    result.ActionType,
		Active:        result.Active,
		LikeCount:     result.LikeCount,
		FavoriteCount: result.FavoriteCount,
	}
}

func commentResponseFromResult(result *applicationinteraction.CreateCommentResult) commentResponse {
	response := commentResponseFromDomain(result.Comment)
	response.CommentCount = result.CommentCount
	return response
}

func commentListResponseFromResult(result *applicationinteraction.CommentListResult) commentListResponse {
	items := make([]commentResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, commentResponseFromDomain(item))
	}
	return commentListResponse{
		Items:      items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
}

func commentResponseFromDomain(comment *domaininteraction.Comment) commentResponse {
	return commentResponse{
		ID:            comment.ID,
		VideoID:       comment.VideoID,
		UserID:        comment.UserID,
		UserNickname:  comment.UserNickname,
		UserAvatarURL: comment.UserAvatarURL,
		Content:       comment.Content,
		CreatedAt:     comment.CreatedAt,
	}
}

func writeInteractionError(c *gin.Context, err error) {
	if isBadRequestError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, domaininteraction.ErrVideoNotFound) || errors.Is(err, domaininteraction.ErrCommentNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "resource not found"})
		return
	}
	if errors.Is(err, domaininteraction.ErrCommentPermissionDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": "comment permission denied"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func isBadRequestError(err error) bool {
	return errors.Is(err, domaininteraction.ErrInvalidUserID) ||
		errors.Is(err, domaininteraction.ErrInvalidVideoID) ||
		errors.Is(err, domaininteraction.ErrInvalidCommentID) ||
		errors.Is(err, domaininteraction.ErrInvalidActionType) ||
		errors.Is(err, domaininteraction.ErrInvalidLimit) ||
		errors.Is(err, domaininteraction.ErrInvalidCursor) ||
		errors.Is(err, domaininteraction.ErrEmptyCommentContent) ||
		errors.Is(err, domaininteraction.ErrCommentContentTooLong) ||
		errors.Is(err, domaininteraction.ErrIdempotencyKeyTooLong)
}
