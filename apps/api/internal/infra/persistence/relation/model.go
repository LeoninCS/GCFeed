package infrarelation

import "time"

// FollowModel 映射 user_follow 表，记录用户之间的关注状态。
type FollowModel struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID         int64     `gorm:"column:user_id;not null;uniqueIndex:uk_user_target,priority:1;index:idx_user_status_updated,priority:1"`
	TargetUserID   int64     `gorm:"column:target_user_id;not null;uniqueIndex:uk_user_target,priority:2;index:idx_target_status_updated,priority:1;index:idx_user_status_updated,priority:4"`
	Status         int       `gorm:"column:status;type:tinyint;not null;default:1;index:idx_user_status_updated,priority:2;index:idx_target_status_updated,priority:2"`
	IdempotencyKey *string   `gorm:"column:idempotency_key;size:128"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime;index:idx_user_status_updated,priority:3;index:idx_target_status_updated,priority:3"`
}

// TableName 指定关注关系表名。
func (FollowModel) TableName() string {
	return "user_follow"
}

// RelationStatModel 映射 user_relation_stat 表，保存关注数和粉丝数。
type RelationStatModel struct {
	UserID         int64     `gorm:"column:user_id;primaryKey"`
	FollowingCount int       `gorm:"column:following_count;not null;default:0"`
	FollowerCount  int       `gorm:"column:follower_count;not null;default:0"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt      time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 指定关系统计表名。
func (RelationStatModel) TableName() string {
	return "user_relation_stat"
}
