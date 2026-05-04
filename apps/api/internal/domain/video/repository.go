package domainvideo

import "context"

// Repository 定义视频领域需要的持久化能力。
type Repository interface {
	// Save 保存视频和初始统计数据。
	Save(ctx context.Context, video *Video) error
	// FindByID 查询已发布视频，用于公开详情。
	FindByID(ctx context.Context, id int64) (*Video, error)
	// FindByIDAnyStatus 查询任意状态视频，用于作者删除等内部场景。
	FindByIDAnyStatus(ctx context.Context, id int64) (*Video, error)
	// FindByAuthorAndIdempotencyKey 用于发布接口的幂等重放。
	FindByAuthorAndIdempotencyKey(ctx context.Context, authorID int64, key string) (*Video, error)
	// ListByAuthor 查询作者已发布视频列表。
	ListByAuthor(ctx context.Context, authorID int64, limit, offset int) ([]*Video, error)
	// UpdateStatus 更新视频状态，例如软删除。
	UpdateStatus(ctx context.Context, video *Video) error
}
