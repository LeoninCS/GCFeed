package infrafeed

import (
	domainfeed "GCFeed/internal/domain/feed"
	domainvideo "GCFeed/internal/domain/video"
	"context"
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

type timelineFeedVideoModel struct {
	VideoID         int64
	AuthorID        int64
	AuthorNickname  string
	AuthorAvatarURL string
	Title           string
	Description     string
	MediaURL        string
	CoverURL        string
	LikeCount       int
	CommentCount    int
	FavoriteCount   int
	PublishedAt     time.Time
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListTimelineFeed(ctx context.Context, cursor *domainfeed.TimelineCursor, limit int) ([]*domainfeed.FeedItem, error) {
	var models []timelineFeedVideoModel
	query := r.db.WithContext(ctx).
		Table("video AS v").
		Select("v.id AS video_id, v.author_id, a.nickname AS author_nickname, a.avatar_url AS author_avatar_url, v.title, v.description, v.media_url, v.cover_url, COALESCE(vs.like_count, 0) AS like_count, COALESCE(vs.comment_count, 0) AS comment_count, COALESCE(vs.favorite_count, 0) AS favorite_count, v.published_at").
		Joins("LEFT JOIN account AS a ON a.id = v.author_id").
		Joins("LEFT JOIN video_stat AS vs ON vs.video_id = v.id").
		Where("v.status = ? AND v.published_at IS NOT NULL", domainvideo.StatusPublished)

	if cursor != nil {
		query = query.Where(
			"(v.published_at < ? OR (v.published_at = ? AND v.id < ?))",
			cursor.PublishedAt,
			cursor.PublishedAt,
			cursor.VideoID,
		)
	}

	err := query.
		Order("v.published_at DESC").
		Order("v.id DESC").
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
			model.AuthorNickname,
			model.AuthorAvatarURL,
			model.Title,
			model.Description,
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
