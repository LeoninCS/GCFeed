package domainfeed

import "context"

type Repository interface {
	ListTimelineFeed(ctx context.Context, cursor *TimelineCursor, limit int) ([]*FeedItem, error)
}
