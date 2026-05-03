package infravideo

import "time"

type VideoModel struct {
	ID             int64      `gorm:"column:id;primaryKey;autoIncrement"`
	AuthorID       int64      `gorm:"column:author_id;not null;index:idx_author_status,priority:1;uniqueIndex:uk_author_idempotency,priority:1"`
	Title          string     `gorm:"column:title;size:128;not null"`
	MediaURL       string     `gorm:"column:media_url;size:512;not null"`
	CoverURL       string     `gorm:"column:cover_url;size:512;not null"`
	Status         int        `gorm:"column:status;type:tinyint;not null;default:2;index:idx_author_status,priority:2;index:idx_status_published,priority:1"`
	LikeCount      int        `gorm:"column:like_count;not null;default:0"`
	CommentCount   int        `gorm:"column:comment_count;not null;default:0"`
	FavoriteCount  int        `gorm:"column:favorite_count;not null;default:0"`
	PublishedAt    *time.Time `gorm:"column:published_at;index:idx_status_published,priority:2"`
	IdempotencyKey *string    `gorm:"column:idempotency_key;size:128;uniqueIndex:uk_author_idempotency,priority:2"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime;index:idx_author_status,priority:3"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime"`
}

func (VideoModel) TableName() string {
	return "video"
}
