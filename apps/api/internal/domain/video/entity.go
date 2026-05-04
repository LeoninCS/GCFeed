package domainvideo

import (
	"strings"
	"time"
)

const (
	StatusDraft     = 1
	StatusPublished = 2
	StatusOffline   = 3
	StatusDeleted   = 4

	MaxTitleLength          = 128
	MaxDescriptionLength    = 512
	MaxIdempotencyKeyLength = 128
)

// Video 是视频聚合根，包含内容信息、发布状态和统计快照。
type Video struct {
	ID             int64
	AuthorID       int64
	Title          string
	Description    string
	MediaURL       string
	CoverURL       string
	Status         int
	LikeCount      int
	CommentCount   int
	FavoriteCount  int
	PublishedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	IdempotencyKey string
}

// NewPublished 创建一个直接发布的视频，适合当前项目的发布流程。
func NewPublished(authorID int64, title, description, mediaURL, coverURL, idempotencyKey string) (*Video, error) {
	if authorID <= 0 {
		return nil, ErrInvalidAuthorID
	}

	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	mediaURL = strings.TrimSpace(mediaURL)
	coverURL = strings.TrimSpace(coverURL)
	idempotencyKey = strings.TrimSpace(idempotencyKey)

	if title == "" {
		return nil, ErrEmptyTitle
	}
	if len(title) > MaxTitleLength {
		return nil, ErrTitleTooLong
	}
	if len(description) > MaxDescriptionLength {
		return nil, ErrDescriptionTooLong
	}
	if mediaURL == "" {
		return nil, ErrEmptyMediaURL
	}
	if coverURL == "" {
		return nil, ErrEmptyCoverURL
	}
	if len(idempotencyKey) > MaxIdempotencyKeyLength {
		return nil, ErrIdempotencyKeyTooLong
	}

	now := time.Now()
	// 新建视频直接进入 Published 状态，同时记录发布时间用于 Feed 排序。
	return &Video{
		AuthorID:       authorID,
		Title:          title,
		Description:    description,
		MediaURL:       mediaURL,
		CoverURL:       coverURL,
		Status:         StatusPublished,
		PublishedAt:    &now,
		IdempotencyKey: idempotencyKey,
	}, nil
}

// RestoreVideo 从数据库查询结果恢复领域对象，统计字段来自 video_stat 表。
func RestoreVideo(
	id int64,
	authorID int64,
	title string,
	description string,
	mediaURL string,
	coverURL string,
	status int,
	likeCount int,
	commentCount int,
	favoriteCount int,
	publishedAt *time.Time,
	createdAt time.Time,
	updatedAt time.Time,
	idempotencyKey string,
) *Video {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	mediaURL = strings.TrimSpace(mediaURL)
	coverURL = strings.TrimSpace(coverURL)
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if status == 0 {
		status = StatusPublished
	}

	return &Video{
		ID:             id,
		AuthorID:       authorID,
		Title:          title,
		Description:    description,
		MediaURL:       mediaURL,
		CoverURL:       coverURL,
		Status:         status,
		LikeCount:      likeCount,
		CommentCount:   commentCount,
		FavoriteCount:  favoriteCount,
		PublishedAt:    publishedAt,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		IdempotencyKey: idempotencyKey,
	}
}

// DeleteBy 执行作者权限校验并把视频置为删除状态。
func (v *Video) DeleteBy(authorID int64) error {
	if authorID <= 0 {
		return ErrInvalidAuthorID
	}
	if v.AuthorID != authorID {
		return ErrVideoPermissionDenied
	}
	// 删除采用软删除，保留原始记录用于审计、统计或后续恢复。
	if v.Status == StatusDeleted {
		return nil
	}
	v.Status = StatusDeleted
	return nil
}
