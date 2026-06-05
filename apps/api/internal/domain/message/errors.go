package domainmessage

import "errors"

// 消息领域错误描述业务规则失败原因，应用层和 HTTP 层基于这些错误做转换。
var ErrInvalidUserID = errors.New("user id must be positive")
var ErrInvalidMessageID = errors.New("message id must be positive")
var ErrInvalidLimit = errors.New("limit must be positive")
var ErrInvalidCursor = errors.New("cursor is invalid")
var ErrInvalidMessageType = errors.New("message type is invalid")
var ErrEmptyTitle = errors.New("title is required")
var ErrTitleTooLong = errors.New("title is too long")
var ErrEmptyContent = errors.New("content is required")
var ErrContentTooLong = errors.New("content is too long")
var ErrEventIDTooLong = errors.New("event id is too long")
var ErrIdempotencyKeyTooLong = errors.New("idempotency key is too long")
var ErrMessageNotFound = errors.New("message not found")
