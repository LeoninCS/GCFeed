package domaininteraction

import "errors"

// 互动领域错误覆盖点赞、收藏、评论和权限判断的业务失败原因。
var ErrInvalidUserID = errors.New("user id must be positive")
var ErrInvalidVideoID = errors.New("video id must be positive")
var ErrInvalidCommentID = errors.New("comment id must be positive")
var ErrInvalidActionType = errors.New("action type is invalid")
var ErrInvalidLimit = errors.New("limit must be positive")
var ErrInvalidCursor = errors.New("cursor is invalid")
var ErrEmptyCommentContent = errors.New("comment content is required")
var ErrCommentContentTooLong = errors.New("comment content is too long")
var ErrIdempotencyKeyTooLong = errors.New("idempotency key is too long")
var ErrVideoNotFound = errors.New("video not found")
var ErrCommentNotFound = errors.New("comment not found")
var ErrCommentPermissionDenied = errors.New("comment permission denied")
