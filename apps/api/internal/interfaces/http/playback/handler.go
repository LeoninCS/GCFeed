package interfaceshttpplayback

import (
	applicationplayback "GCFeed/internal/application/playback"
	domainplayback "GCFeed/internal/domain/playback"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *applicationplayback.Service
}

// New 注入播放优化应用服务。
func New(service *applicationplayback.Service) *Handler {
	return &Handler{service: service}
}

// GetConfig 查询当前客户端播放配置。
func (h *Handler) GetConfig(c *gin.Context) {
	result, err := h.service.GetConfig(c.Request.Context(), c.Query("platform"), c.Query("network_type"))
	if err != nil {
		writePlaybackError(c, err)
		return
	}
	c.JSON(http.StatusOK, configResponseFromResult(result))
}

// ListPreloadVideos 查询 Feed 当前视频之后的预加载资源。
func (h *Handler) ListPreloadVideos(c *gin.Context) {
	currentVideoID, err := parseOptionalInt64(c.Query("current_video_id"))
	if err != nil {
		writePlaybackError(c, domainplayback.ErrInvalidVideoID)
		return
	}
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		writePlaybackError(c, err)
		return
	}

	result, err := h.service.ListPreloadVideos(c.Request.Context(), currentVideoID, limit)
	if err != nil {
		writePlaybackError(c, err)
		return
	}
	c.JSON(http.StatusOK, preloadResponseFromResult(result))
}

// CreateQoSReport 处理 Web 客户端播放质量上报。
func (h *Handler) CreateQoSReport(c *gin.Context) {
	userID, ok := userIDFromContext(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid access token"})
		return
	}
	h.createQoSReport(c, userID)
}

// CreateInternalQoSReport 处理服务间播放质量上报。
func (h *Handler) CreateInternalQoSReport(c *gin.Context) {
	var req createQoSReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.createQoSReportWithRequest(c, req.UserID, req)
}

func (h *Handler) createQoSReport(c *gin.Context, userID int64) {
	var req createQoSReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	h.createQoSReportWithRequest(c, userID, req)
}

func (h *Handler) createQoSReportWithRequest(c *gin.Context, userID int64, req createQoSReportRequest) {
	result, err := h.service.CreateQoSReport(
		c.Request.Context(),
		userID,
		req.VideoID,
		req.FirstFrameMs,
		req.StutterCount,
		req.WatchMs,
		c.GetHeader("Idempotency-Key"),
	)
	if err != nil {
		writePlaybackError(c, err)
		return
	}

	status := http.StatusCreated
	if !result.Created {
		status = http.StatusOK
	}
	c.JSON(status, qosResponseFromResult(result))
}

func userIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(interfaceshttpmiddleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok && userID > 0
}

func parseOptionalInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < 0 {
		return 0, domainplayback.ErrInvalidVideoID
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
		return 0, domainplayback.ErrInvalidLimit
	}
	return limit, nil
}

func configResponseFromResult(result *applicationplayback.ConfigResult) playbackConfigResponse {
	config := result.Config
	return playbackConfigResponse{
		ID:           config.ID,
		Platform:     config.Platform,
		NetworkType:  config.NetworkType,
		PreloadCount: config.PreloadCount,
		BufferMs:     config.BufferMs,
		UpdatedAt:    config.UpdatedAt,
	}
}

func preloadResponseFromResult(result *applicationplayback.PreloadResult) preloadVideosResponse {
	items := make([]preloadVideoResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, preloadVideoResponse{
			VideoID:  item.VideoID,
			MediaURL: item.MediaURL,
			CoverURL: item.CoverURL,
		})
	}
	return preloadVideosResponse{Items: items}
}

func qosResponseFromResult(result *applicationplayback.QoSReportResult) qosReportResponse {
	report := result.Report
	return qosReportResponse{
		ID:           report.ID,
		UserID:       report.UserID,
		VideoID:      report.VideoID,
		FirstFrameMs: report.FirstFrameMs,
		StutterCount: report.StutterCount,
		WatchMs:      report.WatchMs,
		CreatedAt:    report.CreatedAt,
	}
}

func writePlaybackError(c *gin.Context, err error) {
	if isBadRequestError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

func isBadRequestError(err error) bool {
	return errors.Is(err, domainplayback.ErrInvalidUserID) ||
		errors.Is(err, domainplayback.ErrInvalidVideoID) ||
		errors.Is(err, domainplayback.ErrInvalidPlatform) ||
		errors.Is(err, domainplayback.ErrInvalidNetworkType) ||
		errors.Is(err, domainplayback.ErrInvalidLimit) ||
		errors.Is(err, domainplayback.ErrInvalidFirstFrameMs) ||
		errors.Is(err, domainplayback.ErrInvalidStutterCount) ||
		errors.Is(err, domainplayback.ErrInvalidWatchMs) ||
		errors.Is(err, domainplayback.ErrIdempotencyKeyTooLong)
}
