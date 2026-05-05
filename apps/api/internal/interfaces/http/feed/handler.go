package interfaceshttpfeed

import (
	applicationfeed "GCFeed/internal/application/feed"
	domainfeed "GCFeed/internal/domain/feed"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *applicationfeed.Service
}

// New 注入 Feed 应用服务。
func New(service *applicationfeed.Service) *Handler {
	return &Handler{service: service}
}

// Timeline 读取指定 scene 的 Feed，cursor 和 limit 来自 query 参数。
func (h *Handler) Timeline(c *gin.Context) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		writeFeedError(c, err)
		return
	}

	viewerID, _ := viewerIDFromContext(c)
	result, err := h.service.GetFeed(c.Request.Context(), applicationfeed.FeedRequest{
		Scene:    domainfeed.Scene(c.Query("scene")),
		Cursor:   c.Query("cursor"),
		Limit:    limit,
		ViewerID: viewerID,
	})
	if err != nil {
		writeFeedError(c, err)
		return
	}

	c.JSON(http.StatusOK, timelineFeedResponseFromResult(result))
}

// Query 通过请求体接收复杂 Feed 查询参数，适合推荐上下文逐步扩展。
func (h *Handler) Query(c *gin.Context) {
	var req feedQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	limit, err := parseBodyLimit(req.Limit)
	if err != nil {
		writeFeedError(c, err)
		return
	}

	viewerID, _ := viewerIDFromContext(c)
	result, err := h.service.GetFeed(c.Request.Context(), applicationfeed.FeedRequest{
		Scene:         domainfeed.Scene(req.Scene),
		Cursor:        req.Cursor,
		Limit:         limit,
		ViewerID:      viewerID,
		ClientContext: req.ClientContext,
	})
	if err != nil {
		writeFeedError(c, err)
		return
	}

	c.JSON(http.StatusOK, timelineFeedResponseFromResult(result))
}

// Refresh 从第一页重新读取 Feed，适合下拉刷新语义。
func (h *Handler) Refresh(c *gin.Context) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		writeFeedError(c, err)
		return
	}

	result, err := h.service.RefreshTimelineFeed(c.Request.Context(), limit)
	if err != nil {
		writeFeedError(c, err)
		return
	}

	c.JSON(http.StatusOK, timelineFeedResponseFromResult(result))
}

// parseLimit 只校验用户传入的 limit，默认值交给应用服务处理。
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

// parseBodyLimit 校验 JSON 请求体中的 limit，空值交给应用服务使用默认页大小。
func parseBodyLimit(value *int) (int, error) {
	if value == nil {
		return 0, nil
	}
	if *value <= 0 {
		return 0, domainfeed.ErrInvalidLimit
	}
	return *value, nil
}

// timelineFeedResponseFromResult 把应用层 Feed 结果转换为 HTTP 响应结构。
func timelineFeedResponseFromResult(result *applicationfeed.TimelineFeedResult) timelineFeedResponse {
	items := make([]feedItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, feedItemResponse{
			VideoID:         item.VideoID,
			AuthorID:        item.AuthorID,
			AuthorNickname:  item.AuthorNickname,
			AuthorAvatarURL: item.AuthorAvatarURL,
			Title:           item.Title,
			Description:     item.Description,
			MediaURL:        item.MediaURL,
			CoverURL:        item.CoverURL,
			LikeCount:       item.LikeCount,
			CommentCount:    item.CommentCount,
			FavoriteCount:   item.FavoriteCount,
			PublishedAt:     item.PublishedAt,
		})
	}
	return timelineFeedResponse{
		Scene:      string(result.Scene),
		Items:      items,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
	}
}

// writeFeedError 统一 Feed 接口错误响应。
func writeFeedError(c *gin.Context, err error) {
	if isBadRequestError(err) {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

// isBadRequestError 判断 Feed 参数错误。
func isBadRequestError(err error) bool {
	return errors.Is(err, domainfeed.ErrInvalidLimit) ||
		errors.Is(err, domainfeed.ErrInvalidCursor) ||
		errors.Is(err, domainfeed.ErrUnsupportedScene)
}

// viewerIDFromContext 读取可选登录用户 ID，个性化 Feed 策略可以使用。
func viewerIDFromContext(c *gin.Context) (int64, bool) {
	value, exists := c.Get(interfaceshttpmiddleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok && userID > 0
}
