package domaininteraction

import "context"

// Repository 定义互动领域需要的持久化能力。
type Repository interface {
	// SetAction 设置点赞或收藏状态，并返回最新统计值。
	SetAction(ctx context.Context, userID int64, videoID int64, actionType string, active bool, idempotencyKey string) (*Action, int, error)
	// CreateComment 创建评论并返回视频最新评论数。
	CreateComment(ctx context.Context, comment *Comment) (*Comment, int, error)
	// FindCommentByUserAndIdempotencyKey 用于评论创建接口的幂等重放。
	FindCommentByUserAndIdempotencyKey(ctx context.Context, userID int64, idempotencyKey string) (*Comment, int, error)
	// ListComments 按创建时间倒序读取评论列表。
	ListComments(ctx context.Context, videoID int64, cursor *CommentCursor, limit int) ([]*Comment, error)
	// DeleteComment 软删除评论，并根据操作者身份判断权限。
	DeleteComment(ctx context.Context, commentID int64, userID int64, role string) (*Comment, int, error)
}
