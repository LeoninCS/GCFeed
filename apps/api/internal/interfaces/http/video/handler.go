package interfaceshttpvideo

import (
	"errors"
	applicationvideo "feedsystem_video_hard/internal/application/video"
	domainvideo "feedsystem_video_hard/internal/domain/video"
	interfaceshttpmiddleware "feedsystem_video_hard/internal/interfaces/http/middleware"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const defaultListLimit = 20

type Handler struct {
	service *applicationvideo.Service
}

func NewHandler(service *applicationvideo.Service) *Handler {
	return &Handler{service: service}
}

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

	result, err := h.service.CreatePublished(
		c.Request.Context(),
		userID,
		req.Title,
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
		status = http.StatusOK
	}
	c.JSON(status, videoResponseFromDomain(result.Video))
}

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

func userIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(interfaceshttpmiddleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok && userID > 0
}

func parsePositiveInt64(raw string) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil || value <= 0 {
		return 0, domainvideo.ErrInvalidVideoID
	}
	return value, nil
}

func parsePagination(c *gin.Context) (int, int, error) {
	limit := defaultListLimit
	offset := 0

	rawLimit := strings.TrimSpace(c.Query("limit"))
	if rawLimit != "" {
		value, err := strconv.Atoi(rawLimit)
		if err != nil || value <= 0 {
			return 0, 0, domainvideo.ErrInvalidLimit
		}
		limit = value
	}

	rawOffset := strings.TrimSpace(c.Query("offset"))
	if rawOffset != "" {
		value, err := strconv.Atoi(rawOffset)
		if err != nil || value < 0 {
			return 0, 0, domainvideo.ErrInvalidOffset
		}
		offset = value
	}

	return limit, offset, nil
}

func videoResponseFromDomain(video *domainvideo.Video) videoResponse {
	return videoResponse{
		ID:            video.ID,
		AuthorID:      video.AuthorID,
		Title:         video.Title,
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

func isBadRequestError(err error) bool {
	return errors.Is(err, domainvideo.ErrInvalidVideoID) ||
		errors.Is(err, domainvideo.ErrInvalidAuthorID) ||
		errors.Is(err, domainvideo.ErrEmptyTitle) ||
		errors.Is(err, domainvideo.ErrTitleTooLong) ||
		errors.Is(err, domainvideo.ErrEmptyMediaURL) ||
		errors.Is(err, domainvideo.ErrEmptyCoverURL) ||
		errors.Is(err, domainvideo.ErrIdempotencyKeyTooLong) ||
		errors.Is(err, domainvideo.ErrInvalidLimit) ||
		errors.Is(err, domainvideo.ErrInvalidOffset)
}
