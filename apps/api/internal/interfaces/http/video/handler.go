package interfaceshttpvideo

import (
	applicationvideo "GCFeed/internal/application/video"
	domainvideo "GCFeed/internal/domain/video"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultListLimit = 20

type Handler struct {
	service *applicationvideo.Service
}

// New 注入视频应用服务。
func New(service *applicationvideo.Service) *Handler {
	return &Handler{service: service}
}

// Create 处理发布视频请求，用户身份来自 JWT，上行数据来自 JSON 请求体。
func (h *Handler) Create(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	var req CreateVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Idempotency-Key 来自请求头，用于客户端重试时获得同一个视频结果。
	result, err := h.service.CreatePublished(
		c.Request.Context(),
		userID,
		req.Title,
		req.Description,
		req.MediaURL,
		req.CoverURL,
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		writeVideoError(c, err)
		return
	}

	status := http.StatusCreated
	if !result.Created {
		// 幂等重放返回已有资源，使用 200 表示本次没有新建记录。
		status = http.StatusOK
	}
	c.JSON(status, videoResponseFromDomain(result.Video))
}

// Get 查询公开视频详情，videoId 来自 RESTful 路径参数。
func (h *Handler) Get(c *gin.Context) {
	videoID, err := parsePositiveInt64(c.Param("videoId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid video id"})
		return
	}

	video, err := h.service.Get(c.Request.Context(), videoID)
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusOK, videoResponseFromDomain(video))
}

// Delete 删除当前用户自己的视频，删除操作在领域层做作者权限校验。
func (h *Handler) Delete(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	videoID, err := parsePositiveInt64(c.Param("videoId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid video id"})
		return
	}

	if err := h.service.Delete(c.Request.Context(), userID, videoID); err != nil {
		writeVideoError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ListByAuthor 查询指定用户的公开作品列表。
func (h *Handler) ListByAuthor(c *gin.Context) {
	authorID, err := parsePositiveInt64(c.Param("userId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}

	limit, offset, err := parsePagination(c)
	if err != nil {
		writeVideoError(c, err)
		return
	}

	videos, err := h.service.ListByAuthor(c.Request.Context(), authorID, limit, offset)
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusOK, videoListResponseFromDomain(videos, limit, offset))
}

// ListMine 查询当前登录用户自己的作品列表。
func (h *Handler) ListMine(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	limit, offset, err := parsePagination(c)
	if err != nil {
		writeVideoError(c, err)
		return
	}

	videos, err := h.service.ListByAuthor(c.Request.Context(), userID, limit, offset)
	if err != nil {
		writeVideoError(c, err)
		return
	}

	c.JSON(http.StatusOK, videoListResponseFromDomain(videos, limit, offset))
}

// userIDFromContext 从 JWT 中间件写入的上下文读取登录用户 ID。
func userIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(interfaceshttpmiddleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok && userID > 0
}

// parsePositiveInt64 解析 RESTful 路径中的正整数 ID。
func parsePositiveInt64(raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, domainvideo.ErrInvalidVideoID
	}
	return value, nil
}

// parsePagination 解析 offset 分页参数，默认 limit 在 Handler 层给出。
func parsePagination(c *gin.Context) (int, int, error) {
	limit := defaultListLimit
	offset := 0

	rawLimit := strings.TrimSpace(c.Query("limit"))
	if rawLimit != "" {
		// limit 必须为正数，应用层会进一步限制最大值。
		value, err := strconv.Atoi(rawLimit)
		if err != nil || value <= 0 {
			return 0, 0, domainvideo.ErrInvalidLimit
		}
		limit = value
	}

	rawOffset := strings.TrimSpace(c.Query("offset"))
	if rawOffset != "" {
		// offset 允许为 0，表示从第一条开始。
		value, err := strconv.Atoi(rawOffset)
		if err != nil || value < 0 {
			return 0, 0, domainvideo.ErrInvalidOffset
		}
		offset = value
	}

	return limit, offset, nil
}

// videoResponseFromDomain 把领域视频转换成 HTTP JSON 响应。
func videoResponseFromDomain(video *domainvideo.Video) videoResponse {
	return videoResponse{
		ID:            video.ID,
		AuthorID:      video.AuthorID,
		Title:         video.Title,
		Description:   video.Description,
		MediaURL:      video.MediaURL,
		CoverURL:      video.CoverURL,
		Status:        video.Status,
		LikeCount:     video.LikeCount,
		CommentCount:  video.CommentCount,
		FavoriteCount: video.FavoriteCount,
		PublishedAt:   video.PublishedAt,
		CreatedAt:     video.CreatedAt,
		UpdatedAt:     video.UpdatedAt,
	}
}

// videoListResponseFromDomain 组装列表响应，并回显本次分页参数。
func videoListResponseFromDomain(videos []*domainvideo.Video, limit, offset int) videoListResponse {
	items := make([]videoResponse, 0, len(videos))
	for _, video := range videos {
		items = append(items, videoResponseFromDomain(video))
	}
	return videoListResponse{
		Items:  items,
		Limit:  limit,
		Offset: offset,
	}
}

// writeVideoError 统一视频接口错误到 HTTP 状态码的映射。
func writeVideoError(c *gin.Context, err error) {
	if isBadRequestError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, domainvideo.ErrVideoNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}
	if errors.Is(err, domainvideo.ErrVideoPermissionDenied) {
		c.JSON(http.StatusForbidden, gin.H{"error": "video permission denied"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

// isBadRequestError 判断哪些视频领域错误属于客户端请求问题。
func isBadRequestError(err error) bool {
	return errors.Is(err, domainvideo.ErrInvalidVideoID) ||
		errors.Is(err, domainvideo.ErrInvalidAuthorID) ||
		errors.Is(err, domainvideo.ErrEmptyTitle) ||
		errors.Is(err, domainvideo.ErrTitleTooLong) ||
		errors.Is(err, domainvideo.ErrDescriptionTooLong) ||
		errors.Is(err, domainvideo.ErrEmptyMediaURL) ||
		errors.Is(err, domainvideo.ErrEmptyCoverURL) ||
		errors.Is(err, domainvideo.ErrIdempotencyKeyTooLong) ||
		errors.Is(err, domainvideo.ErrInvalidLimit) ||
		errors.Is(err, domainvideo.ErrInvalidOffset)
}
