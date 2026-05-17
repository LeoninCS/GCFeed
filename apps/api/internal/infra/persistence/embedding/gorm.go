package infraembedding

import (
	domainembedding "GCFeed/internal/domain/embedding"
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// SaveVideoEmbedding 使用 video_id + model upsert，重复发布事件会覆盖同模型向量。
func (r *Repository) SaveVideoEmbedding(ctx context.Context, embedding *domainembedding.VideoEmbedding) error {
	model := VideoEmbeddingModel{
		VideoID:       embedding.VideoID,
		Model:         embedding.Model,
		Dimension:     embedding.Dimension,
		EmbeddingJSON: embedding.EmbeddingJSON,
		TextHash:      embedding.TextHash,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "video_id"},
			{Name: "model"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"dimension",
			"embedding_json",
			"text_hash",
			"updated_at",
		}),
	}).Create(&model).Error
}

// FindVideoEmbedding 按 video_id + model 查询视频向量。
func (r *Repository) FindVideoEmbedding(ctx context.Context, videoID int64, model string) (*domainembedding.VideoEmbedding, error) {
	var item VideoEmbeddingModel
	err := r.db.WithContext(ctx).
		Where("video_id = ? AND model = ?", videoID, model).
		Take(&item).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainembedding.ErrVideoEmbeddingNotFound
		}
		return nil, err
	}
	return domainembedding.RestoreVideoEmbedding(
		item.VideoID,
		item.Model,
		item.Dimension,
		item.EmbeddingJSON,
		item.TextHash,
		item.CreatedAt,
		item.UpdatedAt,
	), nil
}
