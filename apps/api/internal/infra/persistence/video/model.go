package infravideo

import "time"

// VideoModel 映射 video 表，保存视频主体信息和发布状态。
type VideoModel struct {
	ID          int64      `gorm:"column:id;primaryKey;autoIncrement"`
	AuthorID    int64      `gorm:"column:author_id;not null;index:idx_author_status,priority:1;uniqueIndex:uk_author_idempotency,priority:1"`
	Title       string     `gorm:"column:title;size:128;not null"`
	Description string     `gorm:"column:description;size:512"`
	MediaURL    string     `gorm:"column:media_url;size:512;not null"`
	CoverURL    string     `gorm:"column:cover_url;size:512;not null"`
	Status      int        `gorm:"column:status;type:tinyint;not null;default:2;index:idx_author_status,priority:2;index:idx_status_published,priority:1"`
	PublishedAt *time.Time `gorm:"column:published_at;index:idx_status_published,priority:2"`
	// IdempotencyKey 与 AuthorID 组成唯一索引，用于发布接口的安全重试。
	IdempotencyKey *string   `gorm:"column:idempotency_key;size:128;uniqueIndex:uk_author_idempotency,priority:2"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime;index:idx_author_status,priority:3"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 指定数据库表名。
func (VideoModel) TableName() string {
	return "video"
}

// VideoStatModel 映射 video_stat 表，保存可频繁变更的互动计数。
type VideoStatModel struct {
	VideoID       int64     `gorm:"column:video_id;primaryKey"`
	LikeCount     int       `gorm:"column:like_count;not null;default:0"`
	CommentCount  int       `gorm:"column:comment_count;not null;default:0"`
	FavoriteCount int       `gorm:"column:favorite_count;not null;default:0"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 指定统计表名。
func (VideoStatModel) TableName() string {
	return "video_stat"
}
