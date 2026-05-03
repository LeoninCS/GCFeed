package infrafeed

import "time"

type FeedCursorModel struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID      *int64    `gorm:"column:user_id;index:uk_user_scene,priority:1"`
	VisitorID   string    `gorm:"column:visitor_id;size:128;index:uk_user_scene,priority:2"`
	Scene       string    `gorm:"column:scene;size:32;not null;index:uk_user_scene,priority:3"`
	CursorToken string    `gorm:"column:cursor_token;size:255;not null;uniqueIndex:uk_cursor_token"`
	ExpiredAt   time.Time `gorm:"column:expired_at;not null"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (FeedCursorModel) TableName() string {
	return "feed_cursor"
}

type FeedViewEventModel struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID         *int64    `gorm:"column:user_id;index:idx_user_time,priority:1"`
	VisitorID      string    `gorm:"column:visitor_id;size:128;index:idx_visitor_time,priority:1"`
	VideoID        int64     `gorm:"column:video_id;not null;index:idx_video_time,priority:1"`
	EventType      string    `gorm:"column:event_type;size:16;not null"`
	WatchMS        int       `gorm:"column:watch_ms;not null;default:0"`
	IdempotencyKey *string   `gorm:"column:idempotency_key;size:128;uniqueIndex:uk_idempotency_key"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime;index:idx_user_time,priority:2;index:idx_video_time,priority:2;index:idx_visitor_time,priority:2"`
}

func (FeedViewEventModel) TableName() string {
	return "feed_view_event"
}
