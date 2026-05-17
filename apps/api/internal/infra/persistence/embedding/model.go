package infraembedding

import "time"

// VideoEmbeddingModel 映射 video_embedding 表，保存视频内容向量。
type VideoEmbeddingModel struct {
	VideoID       int64     `gorm:"column:video_id;primaryKey;autoIncrement:false;uniqueIndex:uk_video_model,priority:1"`
	Model         string    `gorm:"column:model;size:64;not null;primaryKey;uniqueIndex:uk_video_model,priority:2;index:idx_model_updated,priority:1"`
	Dimension     int       `gorm:"column:dimension;not null"`
	EmbeddingJSON string    `gorm:"column:embedding_json;type:json;not null"`
	TextHash      string    `gorm:"column:text_hash;size:64;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt     time.Time `gorm:"column:updated_at;autoUpdateTime;index:idx_model_updated,priority:2"`
}

// TableName 指定视频向量表名。
func (VideoEmbeddingModel) TableName() string {
	return "video_embedding"
}
