package domainrelation

import "errors"

// 关系领域错误覆盖关注、取关、列表分页和目标用户校验。
var ErrInvalidUserID = errors.New("user id must be positive")
var ErrInvalidTargetUserID = errors.New("target user id must be positive")
var ErrFollowSelfForbidden = errors.New("follow self forbidden")
var ErrInvalidLimit = errors.New("limit must be positive")
var ErrInvalidCursor = errors.New("cursor is invalid")
var ErrIdempotencyKeyTooLong = errors.New("idempotency key is too long")
var ErrTargetUserNotFound = errors.New("target user not found")
var ErrFollowNotFound = errors.New("follow not found")
