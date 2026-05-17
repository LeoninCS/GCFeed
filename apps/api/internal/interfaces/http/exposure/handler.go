package interfaceshttpexposure

import (
	applicationexposure "GCFeed/internal/application/exposure"
	domainexposure "GCFeed/internal/domain/exposure"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *applicationexposure.Service
}

// New 注入曝光应用服务。
func New(service *applicationexposure.Service) *Handler {
	return &Handler{service: service}
}

// CreateViewEvent 处理视频曝光和观看行为上报。
func (h *Handler) CreateViewEvent(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}

	var req createViewEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	result, err := h.service.RecordViewEvent(
		c.Request.Context(),
		userID,
		req.VideoID,
		req.Scene,
		req.RequestID,
		req.EventType,
		req.WatchMs,
		req.Completed,
	)
	if err != nil {
		writeExposureError(c, err)
		return
	}

	c.JSON(http.StatusCreated, responseFromResult(result))
}

func responseFromResult(result *applicationexposure.RecordViewEventResult) createViewEventResponse {
	response := createViewEventResponse{
		Event: viewEventResponse{
			ID:        result.Event.ID,
			UserID:    result.Event.UserID,
			VideoID:   result.Event.VideoID,
			Scene:     result.Event.Scene,
			RequestID: result.Event.RequestID,
			EventType: result.Event.EventType,
			WatchMs:   result.Event.WatchMs,
			Completed: result.Event.Completed,
			CreatedAt: result.Event.CreatedAt,
		},
	}
	if result.Exposure != nil {
		response.Exposure = &exposureResponse{
			UserID:         result.Exposure.UserID,
			VideoID:        result.Exposure.VideoID,
			FirstExposedAt: result.Exposure.FirstExposedAt,
			LastExposedAt:  result.Exposure.LastExposedAt,
			ExposureCount:  result.Exposure.ExposureCount,
			LastScene:      result.Exposure.LastScene,
		}
	}
	return response
}

func userIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(interfaceshttpmiddleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok && userID > 0
}

func writeExposureError(c *gin.Context, err error) {
	if isBadRequestError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if errors.Is(err, domainexposure.ErrVideoNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func isBadRequestError(err error) bool {
	return errors.Is(err, domainexposure.ErrInvalidUserID) ||
		errors.Is(err, domainexposure.ErrInvalidVideoID) ||
		errors.Is(err, domainexposure.ErrEmptyScene) ||
		errors.Is(err, domainexposure.ErrSceneTooLong) ||
		errors.Is(err, domainexposure.ErrInvalidEventType) ||
		errors.Is(err, domainexposure.ErrRequestIDTooLong) ||
		errors.Is(err, domainexposure.ErrWatchMsNegative)
}
