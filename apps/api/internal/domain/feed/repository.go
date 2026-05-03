package domainfeed

import "context"

type Repository interface {
	ListTimeFeed(ctx context.Context, cursor *TimeCursor, limit int) ([]*FeedItem, error)
	SaveViewEvent(ctx context.Context, event *ViewEvent) error
	FindViewEventByIdempotencyKey(ctx context.Context, key string) (*ViewEvent, error)
}
