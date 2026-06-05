package infraplayback

import (
	domainplayback "GCFeed/internal/domain/playback"
	domainvideo "GCFeed/internal/domain/video"
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

// New 创建播放优化仓储实现。
func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// FindConfig 按端和网络类型读取配置。
func (r *Repository) FindConfig(ctx context.Context, platform string, networkType string) (*domainplayback.Config, error) {
	var model ConfigModel
	err := r.db.WithContext(ctx).
		Where("platform = ? AND network_type = ?", platform, networkType).
		Take(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return restoreConfig(model), nil
}

// ListPreloadVideos 读取当前视频之后的公开资源；currentVideoID 为空时返回最新资源。
func (r *Repository) ListPreloadVideos(ctx context.Context, currentVideoID int64, limit int) ([]*domainplayback.PreloadVideo, error) {
	query := r.db.WithContext(ctx).
		Table("video AS v").
		Select("v.id AS video_id, v.media_url, v.cover_url").
		Where("v.status = ? AND v.published_at IS NOT NULL", domainvideo.StatusPublished)

	if currentVideoID > 0 {
		current, err := r.findCurrentVideo(ctx, currentVideoID)
		if err != nil {
			return nil, err
		}
		if current != nil {
			query = query.Where(
				"(v.published_at < ? OR (v.published_at = ? AND v.id < ?))",
				current.PublishedAt,
				current.PublishedAt,
				current.VideoID,
			)
		}
	}

	var models []PreloadVideoModel
	err := query.
		Order("v.published_at DESC").
		Order("v.id DESC").
		Limit(limit).
		Scan(&models).
		Error
	if err != nil {
		return nil, err
	}

	items := make([]*domainplayback.PreloadVideo, 0, len(models))
	for _, model := range models {
		items = append(items, domainplayback.RestorePreloadVideo(model.VideoID, model.MediaURL, model.CoverURL))
	}
	return items, nil
}

// CreateQoSReport 保存播放质量流水，支持 user_id + idempotency_key 幂等。
func (r *Repository) CreateQoSReport(ctx context.Context, report *domainplayback.QoSReport) (*domainplayback.QoSReport, bool, error) {
	model := QoSLogModel{
		UserID:         report.UserID,
		VideoID:        report.VideoID,
		FirstFrameMs:   report.FirstFrameMs,
		StutterCount:   report.StutterCount,
		WatchMs:        report.WatchMs,
		IdempotencyKey: optionalString(report.IdempotencyKey),
	}

	err := r.db.WithContext(ctx).Create(&model).Error
	if err == nil {
		return restoreQoSReport(model), true, nil
	}
	if !isDuplicateKeyError(err) {
		return nil, false, err
	}

	existing, findErr := r.findExistingQoS(ctx, report.UserID, report.IdempotencyKey)
	if findErr != nil {
		return nil, false, findErr
	}
	return restoreQoSReport(existing), false, nil
}

func (r *Repository) findCurrentVideo(ctx context.Context, videoID int64) (*currentVideoModel, error) {
	var model currentVideoModel
	err := r.db.WithContext(ctx).
		Table("video AS v").
		Select("v.id AS video_id, v.published_at").
		Where("v.id = ? AND v.status = ? AND v.published_at IS NOT NULL", videoID, domainvideo.StatusPublished).
		Take(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &model, nil
}

func (r *Repository) findExistingQoS(ctx context.Context, userID int64, idempotencyKey string) (QoSLogModel, error) {
	var model QoSLogModel
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if idempotencyKey == "" {
		return model, gorm.ErrRecordNotFound
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND idempotency_key = ?", userID, idempotencyKey).
		Order("id DESC").
		Take(&model).
		Error
	return model, err
}

type currentVideoModel struct {
	VideoID     int64
	PublishedAt time.Time
}

func restoreConfig(model ConfigModel) *domainplayback.Config {
	return domainplayback.RestoreConfig(model.ID, model.Platform, model.NetworkType, model.PreloadCount, model.BufferMs, model.UpdatedAt)
}

func restoreQoSReport(model QoSLogModel) *domainplayback.QoSReport {
	return domainplayback.RestoreQoSReport(
		model.ID,
		model.UserID,
		model.VideoID,
		model.FirstFrameMs,
		model.StutterCount,
		model.WatchMs,
		stringValue(model.IdempotencyKey),
		model.CreatedAt,
	)
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func isDuplicateKeyError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate")
}
