package domainvideo

import "errors"

var ErrInvalidVideoID = errors.New("video id must be positive")
var ErrInvalidAuthorID = errors.New("author id must be positive")
var ErrEmptyTitle = errors.New("title is required")
var ErrTitleTooLong = errors.New("title is too long")
var ErrDescriptionTooLong = errors.New("description is too long")
var ErrEmptyMediaURL = errors.New("media url is required")
var ErrEmptyCoverURL = errors.New("cover url is required")
var ErrIdempotencyKeyTooLong = errors.New("idempotency key is too long")
var ErrInvalidLimit = errors.New("limit must be positive")
var ErrInvalidOffset = errors.New("offset must be non-negative")
var ErrVideoNotFound = errors.New("video not found")
var ErrVideoPermissionDenied = errors.New("video permission denied")
var ErrDuplicateIdempotencyKey = errors.New("duplicate idempotency key")
