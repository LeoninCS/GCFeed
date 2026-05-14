package infravideo

import (
	domainfeed "GCFeed/internal/domain/feed"
	domainrelation "GCFeed/internal/domain/relation"
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

// videoWithStatModel 承接 video 与 video_stat 联表查询结果。
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

// New 创建视频仓储实现。
func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// EnsureStats 确保每个视频都有一条统计记录。
func EnsureStats(db *gorm.DB) error {
	return db.Exec(`
		INSERT INTO video_stat (video_id, like_count, comment_count, favorite_count, created_at, updated_at)
		SELECT v.id, 0, 0, 0, NOW(), NOW()
		FROM video AS v
		LEFT JOIN video_stat AS vs ON vs.video_id = v.id
		WHERE vs.video_id IS NULL
	`).Error
}

// Save 在同一事务内写入视频记录和初始统计记录。
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

		// video_stat 独立存储计数，便于互动接口只更新统计表。
		stat := VideoStatModel{
			VideoID:       model.ID,
			LikeCount:     video.LikeCount,
			CommentCount:  video.CommentCount,
			FavoriteCount: video.FavoriteCount,
		}
		if err := tx.Create(&stat).Error; err != nil {
			return err
		}
		if err := fanoutPublishedVideoToFollowers(tx, model.AuthorID, model.ID, model.PublishedAt); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 写回数据库生成的 ID 和时间字段，保证返回响应包含完整信息。
	video.ID = model.ID
	video.CreatedAt = model.CreatedAt
	video.UpdatedAt = model.UpdatedAt
	return nil
}

func fanoutPublishedVideoToFollowers(tx *gorm.DB, authorID int64, videoID int64, publishedAt *time.Time) error {
	if publishedAt == nil {
		return nil
	}

	followerCount, err := authorFollowerCount(tx, authorID)
	if err != nil {
		return err
	}
	if followerCount >= domainfeed.BigCreatorFollowerThreshold {
		return nil
	}

	return tx.Exec(`
		INSERT INTO feed_inbox (user_id, video_id, author_id, published_at, created_at)
		SELECT f.user_id, ?, ?, ?, NOW()
		FROM user_follow AS f
		WHERE f.target_user_id = ? AND f.status = ?
		ON DUPLICATE KEY UPDATE published_at = VALUES(published_at)
	`, videoID, authorID, *publishedAt, authorID, domainrelation.FollowStatusActive).Error
}

func authorFollowerCount(tx *gorm.DB, authorID int64) (int, error) {
	var stat struct {
		FollowerCount int
	}
	err := tx.Table("user_relation_stat").
		Select("follower_count").
		Where("user_id = ?", authorID).
		Take(&stat).
		Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return stat.FollowerCount, err
}

// FindByID 查询公开可见的视频详情，只返回 Published 状态。
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

// FindByIDAnyStatus 查询任意状态视频，供作者删除等内部流程使用。
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

// FindByAuthorAndIdempotencyKey 根据作者和幂等键查找已创建视频。
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

// ListByAuthor 按发布时间倒序返回作者已发布视频。
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
		// 查询模型逐条恢复为领域对象，应用层无需知道数据库联表细节。
		videos = append(videos, restoreVideo(model))
	}
	return videos, nil
}

// UpdateStatus 只更新状态字段，用于软删除。
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

// restoreVideo 把联表查询结果转换成领域视频对象。
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

// videoWithStatSelect 统一视频详情查询字段，避免多个查询写重复 SQL 字段列表。
func videoWithStatSelect() string {
	return "v.id, v.author_id, v.title, v.description, v.media_url, v.cover_url, v.status, COALESCE(vs.like_count, 0) AS like_count, COALESCE(vs.comment_count, 0) AS comment_count, COALESCE(vs.favorite_count, 0) AS favorite_count, v.published_at, v.idempotency_key, v.created_at, v.updated_at"
}

// idempotencyKeyPtr 将空幂等键存为 NULL，配合唯一索引允许普通创建多次执行。
func idempotencyKeyPtr(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// idempotencyKeyValue 将数据库可空字段还原成领域层字符串。
func idempotencyKeyValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// isDuplicateKeyError 兼容 GORM 标准错误和 MySQL 1062 唯一键冲突。
func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
