package infraexposure

import "time"

// ViewEventModel 映射 video_view_events 表，保存观看行为流水。
type ViewEventModel struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID    int64     `gorm:"column:user_id;not null;index:idx_user_created,priority:1"`
	VideoID   int64     `gorm:"column:video_id;not null;index:idx_video_created,priority:1"`
	Scene     string    `gorm:"column:scene;size:32;not null;index:idx_user_scene_created,priority:2"`
	RequestID *string   `gorm:"column:request_id;size:64;index:idx_request_event,priority:1"`
	EventType string    `gorm:"column:event_type;size:32;not null;index:idx_request_event,priority:2"`
	WatchMs   int       `gorm:"column:watch_ms;not null;default:0"`
	Completed bool      `gorm:"column:completed;not null;default:false"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime;index:idx_user_created,priority:2;index:idx_video_created,priority:2;index:idx_user_scene_created,priority:3"`
}

// TableName 指定观看行为表名。
func (ViewEventModel) TableName() string {
	return "video_view_events"
}

// ExposureModel 映射 exposures 表，保存用户看过视频的聚合索引。
type ExposureModel struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID         int64     `gorm:"column:user_id;not null;uniqueIndex:uk_user_video,priority:1;index:idx_user_last_exposed,priority:1"`
	VideoID        int64     `gorm:"column:video_id;not null;uniqueIndex:uk_user_video,priority:2;index:idx_video_last_exposed,priority:1"`
	FirstExposedAt time.Time `gorm:"column:first_exposed_at;not null"`
	LastExposedAt  time.Time `gorm:"column:last_exposed_at;not null;index:idx_user_last_exposed,priority:2;index:idx_video_last_exposed,priority:2"`
	ExposureCount  int       `gorm:"column:exposure_count;not null;default:1"`
	LastScene      string    `gorm:"column:last_scene;size:32;not null"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 指定曝光聚合表名。
func (ExposureModel) TableName() string {
	return "exposures"
}
