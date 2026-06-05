package domainplayback

import "errors"

// 播放领域错误用于应用层和 HTTP 层做稳定映射。
var ErrInvalidUserID = errors.New("user id must be non-negative")
var ErrInvalidVideoID = errors.New("video id must be positive")
var ErrInvalidPlatform = errors.New("platform is invalid")
var ErrInvalidNetworkType = errors.New("network type is invalid")
var ErrInvalidLimit = errors.New("limit must be positive")
var ErrInvalidFirstFrameMs = errors.New("first frame ms must be non-negative")
var ErrInvalidStutterCount = errors.New("stutter count must be non-negative")
var ErrInvalidWatchMs = errors.New("watch ms must be non-negative")
var ErrIdempotencyKeyTooLong = errors.New("idempotency key is too long")
