package domainexposure

import "errors"

// 曝光领域错误描述观看上报中的参数和资源状态问题。
var ErrInvalidUserID = errors.New("user id must be positive")
var ErrInvalidVideoID = errors.New("video id must be positive")
var ErrEmptyScene = errors.New("scene is required")
var ErrSceneTooLong = errors.New("scene is too long")
var ErrInvalidEventType = errors.New("event type is invalid")
var ErrRequestIDTooLong = errors.New("request id is too long")
var ErrWatchMsNegative = errors.New("watch ms must be non-negative")
var ErrVideoNotFound = errors.New("video not found")
