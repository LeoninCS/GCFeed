package domainfeed

import "context"

// Repository 定义 Feed 读取需要的持久化能力。
type Repository interface {
	// ListTimelinePage 按发布时间倒序读取轻量 Feed 页。
	ListTimelinePage(ctx context.Context, cursor *TimelineCursor, limit int) ([]*FeedPageItem, error)
	// ListHotPage 按热度分倒序读取轻量 Feed 页。
	ListHotPage(ctx context.Context, cursor *HotCursor, limit int) ([]*FeedPageItem, error)
	// BatchGetFeedCards 批量读取视频卡片展示字段。
	BatchGetFeedCards(ctx context.Context, videoIDs []int64) (map[int64]*FeedCard, error)
	// BatchGetFeedStats 批量读取视频互动计数。
	BatchGetFeedStats(ctx context.Context, videoIDs []int64) (map[int64]*FeedStat, error)
}
