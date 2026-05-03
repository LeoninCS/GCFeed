package infrafeed

import (
	domainfeed "GCFeed/internal/domain/feed"
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

type timeFeedVideoModel struct {
	VideoID       int64
	AuthorID      int64
	Title         string
	MediaURL      string
	CoverURL      string
	LikeCount     int
	CommentCount  int
	FavoriteCount int
	PublishedAt   time.Time
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListTimeFeed(ctx context.Context, cursor *domainfeed.TimeCursor, limit int) ([]*domainfeed.FeedItem, error) {
	var models []timeFeedVideoModel
	query := r.db.WithContext(ctx).
		Table("video").
		Select("id AS video_id, author_id, title, media_url, cover_url, like_count, comment_count, favorite_count, published_at").
		Where("status = ? AND published_at IS NOT NULL", domainvideo.StatusPublished)

	if cursor != nil {
		query = query.Where(
			"(published_at < ? OR (published_at = ? AND id < ?))",
			cursor.PublishedAt,
			cursor.PublishedAt,
			cursor.VideoID,
		)
	}

	err := query.
		Order("published_at DESC").
		Order("id DESC").
		Limit(limit).
		Scan(&models).
		Error
	if err != nil {
		return nil, err
	}

	items := make([]*domainfeed.FeedItem, 0, len(models))
	for _, model := range models {
		items = append(items, domainfeed.RestoreFeedItem(
			model.VideoID,
			model.AuthorID,
			model.Title,
			model.MediaURL,
			model.CoverURL,
			model.LikeCount,
			model.CommentCount,
			model.FavoriteCount,
			model.PublishedAt,
		))
	}
	return items, nil
}

func (r *Repository) SaveViewEvent(ctx context.Context, event *domainfeed.ViewEvent) error {
	model := FeedViewEventModel{
		UserID:         event.UserID,
		VisitorID:      event.VisitorID,
		VideoID:        event.VideoID,
		EventType:      event.EventType,
		WatchMS:        event.WatchMS,
		IdempotencyKey: idempotencyKeyPtr(event.IdempotencyKey),
	}

	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		if isDuplicateKeyError(err) {
			return domainfeed.ErrDuplicateIdempotencyKey
		}
		return err
	}

	event.ID = model.ID
	event.CreatedAt = model.CreatedAt
	return nil
}

func (r *Repository) FindViewEventByIdempotencyKey(ctx context.Context, key string) (*domainfeed.ViewEvent, error) {
	if key == "" {
		return nil, domainfeed.ErrViewEventNotFound
	}

	var model FeedViewEventModel
	err := r.db.WithContext(ctx).
		Where("idempotency_key = ?", key).
		First(&model).
		Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domainfeed.ErrViewEventNotFound
		}
		return nil, err
	}
	return restoreViewEvent(model), nil
}

func restoreViewEvent(model FeedViewEventModel) *domainfeed.ViewEvent {
	return domainfeed.RestoreViewEvent(
		model.ID,
		model.UserID,
		model.VisitorID,
		model.VideoID,
		model.EventType,
		model.WatchMS,
		idempotencyKeyValue(model.IdempotencyKey),
		model.CreatedAt,
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
