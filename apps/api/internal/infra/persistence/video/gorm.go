package infravideo

import (
	"context"
	"errors"
	domainvideo "feedsystem_video_hard/internal/domain/video"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Save(ctx context.Context, video *domainvideo.Video) error {
	model := VideoModel{
		AuthorID:       video.AuthorID,
		Title:          video.Title,
		MediaURL:       video.MediaURL,
		CoverURL:       video.CoverURL,
		Status:         video.Status,
		LikeCount:      video.LikeCount,
		CommentCount:   video.CommentCount,
		FavoriteCount:  video.FavoriteCount,
		PublishedAt:    video.PublishedAt,
		IdempotencyKey: idempotencyKeyPtr(video.IdempotencyKey),
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicateKeyError(err) {
			return domainvideo.ErrDuplicateIdempotencyKey
		}
		return err
	}

	video.ID = model.ID
	video.CreatedAt = model.CreatedAt
	video.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*domainvideo.Video, error) {
	var model VideoModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", id, domainvideo.StatusPublished).
		First(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainvideo.ErrVideoNotFound
		}
		return nil, err
	}
	return restoreVideo(model), nil
}

func (r *Repository) FindByIDAnyStatus(ctx context.Context, id int64) (*domainvideo.Video, error) {
	var model VideoModel
	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainvideo.ErrVideoNotFound
		}
		return nil, err
	}
	return restoreVideo(model), nil
}

func (r *Repository) FindByAuthorAndIdempotencyKey(ctx context.Context, authorID int64, key string) (*domainvideo.Video, error) {
	if key == "" {
		return nil, domainvideo.ErrVideoNotFound
	}

	var model VideoModel
	err := r.db.WithContext(ctx).
		Where("author_id = ? AND idempotency_key = ?", authorID, key).
		First(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainvideo.ErrVideoNotFound
		}
		return nil, err
	}
	return restoreVideo(model), nil
}

func (r *Repository) ListByAuthor(ctx context.Context, authorID int64, limit, offset int) ([]*domainvideo.Video, error) {
	var models []VideoModel
	err := r.db.WithContext(ctx).
		Where("author_id = ? AND status = ?", authorID, domainvideo.StatusPublished).
		Order("published_at DESC").
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&models).
		Error
	if err != nil {
		return nil, err
	}

	videos := make([]*domainvideo.Video, 0, len(models))
	for _, model := range models {
		videos = append(videos, restoreVideo(model))
	}
	return videos, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, video *domainvideo.Video) error {
	result := r.db.WithContext(ctx).
		Model(&VideoModel{}).
		Where("id = ?", video.ID).
		Update("status", video.Status)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainvideo.ErrVideoNotFound
	}
	return nil
}

func restoreVideo(model VideoModel) *domainvideo.Video {
	return domainvideo.RestoreVideo(
		model.ID,
		model.AuthorID,
		model.Title,
		model.MediaURL,
		model.CoverURL,
		model.Status,
		model.LikeCount,
		model.CommentCount,
		model.FavoriteCount,
		model.PublishedAt,
		model.CreatedAt,
		model.UpdatedAt,
		idempotencyKeyValue(model.IdempotencyKey),
	)
}

func idempotencyKeyPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func idempotencyKeyValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
