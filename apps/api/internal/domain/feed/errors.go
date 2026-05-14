package domainfeed

import "errors"

// Feed 领域错误主要来自分页参数和游标解析。
var ErrInvalidLimit = errors.New("limit must be positive")
var ErrInvalidCursor = errors.New("cursor is invalid")
var ErrUnsupportedScene = errors.New("feed scene is unsupported")
var ErrViewerRequired = errors.New("viewer is required")
