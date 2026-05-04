package domainfeed

import "context"

// Repository 定义 Feed 读取需要的持久化能力。
type Repository interface {
	// ListTimelineFeed 按发布时间倒序读取 Feed，cursor 为空时读取第一页。
	ListTimelineFeed(ctx context.Context, cursor *TimelineCursor, limit int) ([]*FeedItem, error)
}
