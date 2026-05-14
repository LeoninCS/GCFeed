package infrafeed

import "time"

// InboxModel 映射关注流收件箱，普通作者发布时推送到粉丝 inbox。
type InboxModel struct {
	ID          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	UserID      int64     `gorm:"column:user_id;not null;uniqueIndex:uk_user_video,priority:1;index:idx_user_published_video,priority:1"`
	VideoID     int64     `gorm:"column:video_id;not null;uniqueIndex:uk_user_video,priority:2;index:idx_video,priority:1;index:idx_user_published_video,priority:3"`
	AuthorID    int64     `gorm:"column:author_id;not null;index:idx_author,priority:1"`
	PublishedAt time.Time `gorm:"column:published_at;not null;index:idx_user_published_video,priority:2"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName 指定关注流收件箱表名。
func (InboxModel) TableName() string {
	return "feed_inbox"
}
