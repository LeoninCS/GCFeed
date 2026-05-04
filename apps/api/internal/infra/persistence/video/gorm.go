package infravideo

import (
	domainvideo "GCFeed/internal/domain/video"
	"context"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type videoWithStatModel struct {
	ID             int64
	AuthorID       int64
	Title          string
	Description    string
	MediaURL       string
	CoverURL       string
	Status         int
	LikeCount      int
	CommentCount   int
	FavoriteCount  int
	PublishedAt    *time.Time
	IdempotencyKey *string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func EnsureStats(db *gorm.DB) error {
	hasLegacyColumns, err := hasLegacyVideoStatColumns(db)
	if err != nil {
		return err
	}

	if hasLegacyColumns {
		return db.Exec(`
			INSERT INTO video_stat (video_id, like_count, comment_count, favorite_count, created_at, updated_at)
			SELECT v.id, v.like_count, v.comment_count, v.favorite_count, NOW(), NOW()
			FROM video AS v
			LEFT JOIN video_stat AS vs ON vs.video_id = v.id
			WHERE vs.video_id IS NULL
		`).Error
	}

	return db.Exec(`
		INSERT INTO video_stat (video_id, like_count, comment_count, favorite_count, created_at, updated_at)
		SELECT v.id, 0, 0, 0, NOW(), NOW()
		FROM video AS v
		LEFT JOIN video_stat AS vs ON vs.video_id = v.id
		WHERE vs.video_id IS NULL
	`).Error
}

func (r *Repository) Save(ctx context.Context, video *domainvideo.Video) error {
	var model VideoModel
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		model = VideoModel{
			AuthorID:       video.AuthorID,
			Title:          video.Title,
			Description:    video.Description,
			MediaURL:       video.MediaURL,
			CoverURL:       video.CoverURL,
			Status:         video.Status,
			PublishedAt:    video.PublishedAt,
			IdempotencyKey: idempotencyKeyPtr(video.IdempotencyKey),
		}

		if err := tx.Create(&model).Error; err != nil {
			if isDuplicateKeyError(err) {
				return domainvideo.ErrDuplicateIdempotencyKey
			}
			return err
		}

		stat := VideoStatModel{
			VideoID:       model.ID,
			LikeCount:     video.LikeCount,
			CommentCount:  video.CommentCount,
			FavoriteCount: video.FavoriteCount,
		}
		if err := tx.Create(&stat).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	video.ID = model.ID
	video.CreatedAt = model.CreatedAt
	video.UpdatedAt = model.UpdatedAt
	return nil
}

func (r *Repository) FindByID(ctx context.Context, id int64) (*domainvideo.Video, error) {
	var model videoWithStatModel
	err := r.db.WithContext(ctx).
		Table("video AS v").
		Select(videoWithStatSelect()).
		Joins("LEFT JOIN video_stat AS vs ON vs.video_id = v.id").
		Where("v.id = ? AND v.status = ?", id, domainvideo.StatusPublished).
		Take(&model).
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
	var model videoWithStatModel
	err := r.db.WithContext(ctx).
		Table("video AS v").
		Select(videoWithStatSelect()).
		Joins("LEFT JOIN video_stat AS vs ON vs.video_id = v.id").
		Where("v.id = ?", id).
		Take(&model).
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

	var model videoWithStatModel
	err := r.db.WithContext(ctx).
		Table("video AS v").
		Select(videoWithStatSelect()).
		Joins("LEFT JOIN video_stat AS vs ON vs.video_id = v.id").
		Where("v.author_id = ? AND v.idempotency_key = ?", authorID, key).
		Take(&model).
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
	var models []videoWithStatModel
	err := r.db.WithContext(ctx).
		Table("video AS v").
		Select(videoWithStatSelect()).
		Joins("LEFT JOIN video_stat AS vs ON vs.video_id = v.id").
		Where("v.author_id = ? AND v.status = ?", authorID, domainvideo.StatusPublished).
		Order("v.published_at DESC").
		Order("v.id DESC").
		Limit(limit).
		Offset(offset).
		Scan(&models).
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

func restoreVideo(model videoWithStatModel) *domainvideo.Video {
	return domainvideo.RestoreVideo(
		model.ID,
		model.AuthorID,
		model.Title,
		model.Description,
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

func videoWithStatSelect() string {
	return "v.id, v.author_id, v.title, v.description, v.media_url, v.cover_url, v.status, COALESCE(vs.like_count, 0) AS like_count, COALESCE(vs.comment_count, 0) AS comment_count, COALESCE(vs.favorite_count, 0) AS favorite_count, v.published_at, v.idempotency_key, v.created_at, v.updated_at"
}

func hasLegacyVideoStatColumns(db *gorm.DB) (bool, error) {
	var count int64
	err := db.Raw(`
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = DATABASE()
			AND table_name = 'video'
			AND column_name IN ('like_count', 'comment_count', 'favorite_count')
	`).Scan(&count).Error
	if err != nil {
		return false, err
	}
	return count == 3, nil
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
