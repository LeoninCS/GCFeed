package infrainteraction

import "time"

type ActionModel struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID         int64     `gorm:"column:user_id;not null;uniqueIndex:uk_user_video_type,priority:1;index:idx_user_type_status,priority:1"`
	VideoID        int64     `gorm:"column:video_id;not null;uniqueIndex:uk_user_video_type,priority:2;index:idx_video_type_status,priority:1"`
	ActionType     string    `gorm:"column:action_type;size:16;not null;uniqueIndex:uk_user_video_type,priority:3;index:idx_video_type_status,priority:2;index:idx_user_type_status,priority:2"`
	Status         int       `gorm:"column:status;type:tinyint;not null;default:1;index:idx_video_type_status,priority:3;index:idx_user_type_status,priority:3"`
	IdempotencyKey *string   `gorm:"column:idempotency_key;size:128"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (ActionModel) TableName() string {
	return "interaction_action"
}

type CommentModel struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	VideoID        int64     `gorm:"column:video_id;not null;index:idx_video_status_created,priority:1"`
	UserID         int64     `gorm:"column:user_id;not null;index:idx_user_created,priority:1;uniqueIndex:uk_user_idempotency,priority:1"`
	Content        string    `gorm:"column:content;size:1000;not null"`
	Status         int       `gorm:"column:status;type:tinyint;not null;default:1;index:idx_video_status_created,priority:2"`
	IdempotencyKey *string   `gorm:"column:idempotency_key;size:128;uniqueIndex:uk_user_idempotency,priority:2"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime;index:idx_video_status_created,priority:3;index:idx_user_created,priority:2"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime;index:idx_video_status_created,priority:4"`
}

func (CommentModel) TableName() string {
	return "interaction_comment"
}
