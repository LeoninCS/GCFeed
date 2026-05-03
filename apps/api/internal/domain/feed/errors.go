package domainfeed

import "errors"

var ErrInvalidVideoID = errors.New("video id must be positive")
var ErrInvalidLimit = errors.New("limit must be positive")
var ErrInvalidCursor = errors.New("cursor is invalid")
var ErrEmptyEventType = errors.New("event type is required")
var ErrInvalidEventType = errors.New("event type is invalid")
var ErrInvalidWatchMS = errors.New("watch_ms must be non-negative")
var ErrVisitorIDTooLong = errors.New("visitor id is too long")
var ErrIdempotencyKeyTooLong = errors.New("idempotency key is too long")
var ErrViewEventNotFound = errors.New("view event not found")
var ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
