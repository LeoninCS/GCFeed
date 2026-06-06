package domainrelation

import "context"

// Repository 定义关系领域需要的持久化能力。
type Repository interface {
	// SetFollow 设置关注或取关状态，并返回当前用户和目标用户的最新计数。
	SetFollow(ctx context.Context, userID int64, targetUserID int64, active bool, idempotencyKey string) (*Follow, *RelationStat, *RelationStat, error)
	// ListFollowing 查询当前用户关注的人。
	ListFollowing(ctx context.Context, userID int64, cursor *ListCursor, limit int) ([]*UserItem, error)
	// ListFollowers 查询关注当前用户的人。
	ListFollowers(ctx context.Context, userID int64, cursor *ListCursor, limit int) ([]*UserItem, error)
	// GetUserProfile 读取用户展示资料，用于关注通知。
	GetUserProfile(ctx context.Context, userID int64) (*UserProfile, error)
}
