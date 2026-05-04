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

// New 注入 Feed 应用服务。
func New(service *applicationfeed.Service) *Handler {
	return &Handler{service: service}
}

// Timeline 读取时间线 Feed，cursor 和 limit 来自 query 参数。
func (h *Handler) Timeline(c *gin.Context) {
	limit, err := parseLimit(c.Query("limit"))
	if err != nil {
		writeFeedError(c, err)
		return
	}

	result, err := h.service.GetTimelineFeed(c.Request.Context(), c.Query("cursor"), limit)
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
		errors.Is(err, domainfeed.ErrInvalidCursor)
}
