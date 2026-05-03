package interfaceshttpfeed

import (
	applicationfeed "GCFeed/internal/application/feed"
	domainfeed "GCFeed/internal/domain/feed"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *applicationfeed.Service
}

func NewHandler(service *applicationfeed.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Time(c *gin.Context) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		writeFeedError(c, err)
		return
	}

	result, err := h.service.GetTimeFeed(c.Request.Context(), c.Query("cursor"), limit)
	if err != nil {
		writeFeedError(c, err)
		return
	}

	c.JSON(http.StatusOK, timeFeedResponseFromResult(result))
}

func (h *Handler) Refresh(c *gin.Context) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		writeFeedError(c, err)
		return
	}

	result, err := h.service.RefreshTimeFeed(c.Request.Context(), limit)
	if err != nil {
		writeFeedError(c, err)
		return
	}

	c.JSON(http.StatusOK, timeFeedResponseFromResult(result))
}

func (h *Handler) ReportViewEvent(c *gin.Context) {
	var req viewEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	result, err := h.service.ReportViewEvent(
		c.Request.Context(),
		nil,
		req.VisitorID,
		req.VideoID,
		req.EventType,
		req.WatchMS,
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		writeFeedError(c, err)
		return
	}

	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}
	c.JSON(status, viewEventResponseFromDomain(result.Event))
}

func parseLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, domainfeed.ErrInvalidLimit
	}
	return limit, nil
}

func timeFeedResponseFromResult(result *applicationfeed.TimeFeedResult) timeFeedResponse {
	items := make([]feedItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, feedItemResponse{
			VideoID:       item.VideoID,
			AuthorID:      item.AuthorID,
			Title:         item.Title,
			MediaURL:      item.MediaURL,
			CoverURL:      item.CoverURL,
			LikeCount:     item.LikeCount,
			CommentCount:  item.CommentCount,
			FavoriteCount: item.FavoriteCount,
			PublishedAt:   item.PublishedAt,
		})
	}
	return timeFeedResponse{
		Items:      items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
}

func viewEventResponseFromDomain(event *domainfeed.ViewEvent) viewEventResponse {
	return viewEventResponse{
		ID:        event.ID,
		VisitorID: event.VisitorID,
		VideoID:   event.VideoID,
		EventType: event.EventType,
		WatchMS:   event.WatchMS,
		CreatedAt: event.CreatedAt,
	}
}

func writeFeedError(c *gin.Context, err error) {
	if isBadRequestError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func isBadRequestError(err error) bool {
	return errors.Is(err, domainfeed.ErrInvalidVideoID) ||
		errors.Is(err, domainfeed.ErrInvalidLimit) ||
		errors.Is(err, domainfeed.ErrInvalidCursor) ||
		errors.Is(err, domainfeed.ErrEmptyEventType) ||
		errors.Is(err, domainfeed.ErrInvalidEventType) ||
		errors.Is(err, domainfeed.ErrInvalidWatchMS) ||
		errors.Is(err, domainfeed.ErrVisitorIDTooLong) ||
		errors.Is(err, domainfeed.ErrIdempotencyKeyTooLong)
}
