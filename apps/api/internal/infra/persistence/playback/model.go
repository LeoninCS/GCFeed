package infraplayback

import "time"

// ConfigModel 映射 playback_config 表，保存端侧播放策略。
type ConfigModel struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Platform     string    `gorm:"column:platform;size:16;not null;uniqueIndex:uk_platform_network,priority:1"`
	NetworkType  string    `gorm:"column:network_type;size:16;not null;uniqueIndex:uk_platform_network,priority:2"`
	PreloadCount int       `gorm:"column:preload_count;not null"`
	BufferMs     int       `gorm:"column:buffer_ms;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 指定配置表名。
func (ConfigModel) TableName() string {
	return "playback_config"
}

// QoSLogModel 映射 playback_qos_log 表，记录播放质量流水。
type QoSLogModel struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID         int64     `gorm:"column:user_id;not null;index:idx_user_video_time,priority:1;uniqueIndex:uk_user_idempotency,priority:1"`
	VideoID        int64     `gorm:"column:video_id;not null;index:idx_video_time,priority:1;index:idx_user_video_time,priority:2"`
	FirstFrameMs   *int      `gorm:"column:first_frame_ms"`
	StutterCount   int       `gorm:"column:stutter_count;not null;default:0"`
	WatchMs        int       `gorm:"column:watch_ms;not null;default:0"`
	IdempotencyKey *string   `gorm:"column:idempotency_key;size:128;uniqueIndex:uk_user_idempotency,priority:2"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime;index:idx_video_time,priority:2;index:idx_user_video_time,priority:3"`
}

// TableName 指定 QoS 流水表名。
func (QoSLogModel) TableName() string {
	return "playback_qos_log"
}

type PreloadVideoModel struct {
	VideoID  int64  `gorm:"column:video_id"`
	MediaURL string `gorm:"column:media_url"`
	CoverURL string `gorm:"column:cover_url"`
}
