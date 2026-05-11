package infrafeed

import (
	domainfeed "GCFeed/internal/domain/feed"
	domainvideo "GCFeed/internal/domain/video"
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
)

const hotScoreExpression = "COALESCE(vs.like_count, 0) * 3 + COALESCE(vs.comment_count, 0) * 5 + COALESCE(vs.favorite_count, 0) * 4"

type Repository struct {
	db *gorm.DB
}

// timelineFeedVideoModel 承接 Feed 联表查询结果。
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
	HotScore        int
	PublishedAt     time.Time
}

// New 创建 Feed 仓储实现。
func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// ListTimelineFeed 查询时间线 Feed，视频、作者和统计数据在这里一次性拼好。
func (r *Repository) ListTimelineFeed(ctx context.Context, cursor *domainfeed.TimelineCursor, limit int) ([]*domainfeed.FeedItem, error) {
	var models []timelineFeedVideoModel
	query := r.baseFeedQuery(ctx)

	if cursor != nil {
		// 游标分页条件和排序字段保持一致：published_at DESC, id DESC。
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
		// 仓储层把数据库查询结果恢复成领域 FeedItem，HTTP 层只处理响应格式。
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

// ListHotFeed 查询热榜 Feed，按互动热度倒序，并使用发布时间和视频 ID 保持稳定排序。
func (r *Repository) ListHotFeed(ctx context.Context, cursor *domainfeed.HotCursor, limit int) ([]*domainfeed.FeedItem, error) {
	var models []timelineFeedVideoModel
	query := r.baseFeedQuery(ctx)

	if cursor != nil {
		query = query.Where(
			fmt.Sprintf("((%[1]s) < ? OR ((%[1]s) = ? AND v.published_at < ?) OR ((%[1]s) = ? AND v.published_at = ? AND v.id < ?))", hotScoreExpression),
			cursor.HotScore,
			cursor.HotScore,
			cursor.PublishedAt,
			cursor.HotScore,
			cursor.PublishedAt,
			cursor.VideoID,
		)
	}

	err := query.
		Order("hot_score DESC").
		Order("v.published_at DESC").
		Order("v.id DESC").
		Limit(limit).
		Scan(&models).
		Error
	if err != nil {
		return nil, err
	}

	return feedItemsFromModels(models), nil
}

func (r *Repository) baseFeedQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("video AS v").
		Select("v.id AS video_id, v.author_id, a.nickname AS author_nickname, a.avatar_url AS author_avatar_url, v.title, v.description, v.media_url, v.cover_url, COALESCE(vs.like_count, 0) AS like_count, COALESCE(vs.comment_count, 0) AS comment_count, COALESCE(vs.favorite_count, 0) AS favorite_count, ("+hotScoreExpression+") AS hot_score, v.published_at").
		Joins("LEFT JOIN account AS a ON a.id = v.author_id").
		Joins("LEFT JOIN video_stat AS vs ON vs.video_id = v.id").
		Where("v.status = ? AND v.published_at IS NOT NULL", domainvideo.StatusPublished)
}

func feedItemsFromModels(models []timelineFeedVideoModel) []*domainfeed.FeedItem {
	items := make([]*domainfeed.FeedItem, 0, len(models))
	for _, model := range models {
		// 仓储层把数据库查询结果恢复成领域 FeedItem，HTTP 层只处理响应格式。
		item := domainfeed.RestoreFeedItem(
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
		)
		item.HotScore = model.HotScore
		items = append(items, item)
	}
	return items
}
