package interfaceshttprelation

import (
	applicationrelation "GCFeed/internal/application/relation"
	domainrelation "GCFeed/internal/domain/relation"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *applicationrelation.Service
}

// New 创建关系 HTTP Handler。
func New(service *applicationrelation.Service) *Handler {
	return &Handler{service: service}
}

// Follow 处理关注用户接口。
func (h *Handler) Follow(c *gin.Context) {
	h.setFollow(c, true)
}

// Unfollow 处理取消关注用户接口。
func (h *Handler) Unfollow(c *gin.Context) {
	h.setFollow(c, false)
}

// ListFollowing 查询当前用户关注列表。
func (h *Handler) ListFollowing(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		writeRelationError(c, err)
		return
	}

	result, err := h.service.ListFollowing(c.Request.Context(), userID, c.Query("cursor"), limit)
	if err != nil {
		writeRelationError(c, err)
		return
	}
	c.JSON(http.StatusOK, relationListResponseFromResult(result))
}

// ListFollowers 查询当前用户粉丝列表。
func (h *Handler) ListFollowers(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		writeRelationError(c, err)
		return
	}

	result, err := h.service.ListFollowers(c.Request.Context(), userID, c.Query("cursor"), limit)
	if err != nil {
		writeRelationError(c, err)
		return
	}
	c.JSON(http.StatusOK, relationListResponseFromResult(result))
}

func (h *Handler) setFollow(c *gin.Context, active bool) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	targetUserID, err := parsePositiveInt64(c.Param("targetUserId"), domainrelation.ErrInvalidTargetUserID)
	if err != nil {
		writeRelationError(c, err)
		return
	}

	var result *applicationrelation.FollowResult
	if active {
		result, err = h.service.Follow(c.Request.Context(), userID, targetUserID, c.GetHeader("Idempotency-Key"))
	} else {
		result, err = h.service.Unfollow(c.Request.Context(), userID, targetUserID, c.GetHeader("Idempotency-Key"))
	}
	if err != nil {
		writeRelationError(c, err)
		return
	}
	c.JSON(http.StatusOK, followResponseFromResult(result))
}

func userIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(interfaceshttpmiddleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok && userID > 0
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
		return 0, domainrelation.ErrInvalidLimit
	}
	return limit, nil
}

func followResponseFromResult(result *applicationrelation.FollowResult) followResponse {
	return followResponse{
		UserID:         result.UserID,
		TargetUserID:   result.TargetUserID,
		Status:         result.Status,
		Following:      result.Following,
		FollowingCount: result.FollowingCount,
		FollowerCount:  result.FollowerCount,
	}
}

func relationListResponseFromResult(result *applicationrelation.ListResult) relationListResponse {
	items := make([]relationUserResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, relationUserResponse{
			UserID:     item.UserID,
			Nickname:   item.Nickname,
			AvatarURL:  item.AvatarURL,
			Bio:        item.Bio,
			FollowedAt: item.FollowedAt,
		})
	}
	return relationListResponse{
		Items:      items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
}

func writeRelationError(c *gin.Context, err error) {
	if isBadRequestError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, domainrelation.ErrTargetUserNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "target user not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func isBadRequestError(err error) bool {
	return errors.Is(err, domainrelation.ErrInvalidUserID) ||
		errors.Is(err, domainrelation.ErrInvalidTargetUserID) ||
		errors.Is(err, domainrelation.ErrFollowSelfForbidden) ||
		errors.Is(err, domainrelation.ErrInvalidLimit) ||
		errors.Is(err, domainrelation.ErrInvalidCursor) ||
		errors.Is(err, domainrelation.ErrIdempotencyKeyTooLong)
}
