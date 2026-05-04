package domaininteraction

import "context"

type Repository interface {
	ToggleAction(ctx context.Context, userID int64, videoID int64, actionType string, idempotencyKey string) (*Action, int, error)
	CreateComment(ctx context.Context, comment *Comment) (*Comment, int, error)
	FindCommentByUserAndIdempotencyKey(ctx context.Context, userID int64, idempotencyKey string) (*Comment, int, error)
	ListComments(ctx context.Context, videoID int64, cursor *CommentCursor, limit int) ([]*Comment, error)
	DeleteComment(ctx context.Context, commentID int64, userID int64, role string) (*Comment, int, error)
}
