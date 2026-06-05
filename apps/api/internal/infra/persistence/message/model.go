package inframessage

import "time"

// MessageModel 映射 user_message 表，记录用户站内通知。
type MessageModel struct {
	ID      int64  `gorm:"column:id;primaryKey;autoIncrement"`
	UserID  int64  `gorm:"column:user_id;not null;index:idx_user_read_created,priority:1;uniqueIndex:uk_user_event,priority:1;uniqueIndex:uk_user_idempotency,priority:1"`
	Type    string `gorm:"column:type;size:16;not null"`
	Title   string `gorm:"column:title;size:128;not null"`
	Content string `gorm:"column:content;size:1024;not null"`
	// EventID 与 UserID 组成唯一索引，用于内部事件重复消费的幂等写入。
	EventID *string `gorm:"column:event_id;size:64;uniqueIndex:uk_user_event,priority:2"`
	// IdempotencyKey 与 UserID 组成唯一索引，用于内部接口重复请求的幂等写入。
	IdempotencyKey *string    `gorm:"column:idempotency_key;size:128;uniqueIndex:uk_user_idempotency,priority:2"`
	IsRead         bool       `gorm:"column:is_read;not null;default:false;index:idx_user_read_created,priority:2"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime;index:idx_user_read_created,priority:3;index:idx_user_created,priority:2"`
	ReadAt         *time.Time `gorm:"column:read_at"`
}

// TableName 指定消息表名。
func (MessageModel) TableName() string {
	return "user_message"
}
