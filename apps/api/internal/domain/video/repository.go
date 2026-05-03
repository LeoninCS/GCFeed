package domainvideo

import "context"

type Repository interface {
	Save(ctx context.Context, video *Video) error
	FindByID(ctx context.Context, id int64) (*Video, error)
	FindByIDAnyStatus(ctx context.Context, id int64) (*Video, error)
	FindByAuthorAndIdempotencyKey(ctx context.Context, authorID int64, key string) (*Video, error)
	ListByAuthor(ctx context.Context, authorID int64, limit, offset int) ([]*Video, error)
	UpdateStatus(ctx context.Context, video *Video) error
}
