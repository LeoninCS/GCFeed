package domainrecommendation

import "errors"

var ErrInvalidUserID = errors.New("invalid user id")
var ErrInvalidVideoID = errors.New("invalid video id")
var ErrInvalidLimit = errors.New("invalid limit")
var ErrEmptyScene = errors.New("scene is required")
var ErrSceneTooLong = errors.New("scene is too long")
var ErrRequestIDTooLong = errors.New("request id is too long")
var ErrInvalidCursor = errors.New("invalid cursor")
var ErrVideoNotFound = errors.New("video not found")
