package infrafeed

import (
	domainfeed "GCFeed/internal/domain/feed"
	domainvideo "GCFeed/internal/domain/video"
	"context"
	"fmt"

	"gorm.io/gorm"
)

const hotScoreExpression = "COALESCE(vs.like_count, 0) * 3 + COALESCE(vs.comment_count, 0) * 5 + COALESCE(vs.favorite_count, 0) * 4"

type Repository struct {
	db *gorm.DB
}

// New 创建 Feed 仓储实现。
func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// EnsureTimelineIndex 创建 timeline 回源查询所需索引。
func EnsureTimelineIndex(db *gorm.DB) error {
	var count int64
	err := db.Raw(
		"SELECT COUNT(1) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = ? AND index_name = ?",
		"video",
		"idx_video_timeline",
	).Scan(&count).Error
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	return db.Exec("CREATE INDEX idx_video_timeline ON video (status, published_at DESC, id DESC)").Error
}

// ListTimelinePage 查询时间线 Feed 轻量页，卡片和计数由应用层批量组装。
func (r *Repository) ListTimelinePage(ctx context.Context, cursor *domainfeed.TimelineCursor, limit int) ([]*domainfeed.FeedPageItem, error) {
	var models []domainfeed.FeedPageItem
	query := r.basePageQuery(ctx)

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
	return feedPageItemsFromModels(models), nil
}

// ListHotPage 查询热榜 Feed 轻量页，按互动热度倒序稳定分页。
func (r *Repository) ListHotPage(ctx context.Context, cursor *domainfeed.HotCursor, limit int) ([]*domainfeed.FeedPageItem, error) {
	var models []domainfeed.FeedPageItem
	query := r.baseHotPageQuery(ctx)

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

	return feedPageItemsFromModels(models), nil
}

// BatchGetFeedCards 批量读取视频卡片展示字段，缓存缺失时由应用层调用。
func (r *Repository) BatchGetFeedCards(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedCard, error) {
	cards := map[int64]*domainfeed.FeedCard{}
	if len(videoIDs) == 0 {
		return cards, nil
	}

	var models []domainfeed.FeedCard
	err := r.db.WithContext(ctx).
		Table("video AS v").
		Select("v.id AS video_id, v.author_id, a.nickname AS author_nickname, a.avatar_url AS author_avatar_url, v.title, v.description, v.media_url, v.cover_url, v.published_at").
		Joins("LEFT JOIN account AS a ON a.id = v.author_id").
		Where("v.id IN ? AND v.status = ? AND v.published_at IS NOT NULL", videoIDs, domainvideo.StatusPublished).
		Scan(&models).
		Error
	if err != nil {
		return nil, err
	}
	for index := range models {
		cards[models[index].VideoID] = &models[index]
	}
	return cards, nil
}

// BatchGetFeedStats 批量读取互动计数，缺失统计记录时按 0 处理。
func (r *Repository) BatchGetFeedStats(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedStat, error) {
	stats := map[int64]*domainfeed.FeedStat{}
	if len(videoIDs) == 0 {
		return stats, nil
	}
	for _, videoID := range videoIDs {
		stats[videoID] = &domainfeed.FeedStat{VideoID: videoID}
	}

	var models []domainfeed.FeedStat
	err := r.db.WithContext(ctx).
		Table("video_stat").
		Select("video_id, like_count, comment_count, favorite_count").
		Where("video_id IN ?", videoIDs).
		Scan(&models).
		Error
	if err != nil {
		return nil, err
	}
	for index := range models {
		stats[models[index].VideoID] = &models[index]
	}
	return stats, nil
}

func (r *Repository) basePageQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("video AS v").
		Select("v.id AS video_id, v.published_at").
		Where("v.status = ? AND v.published_at IS NOT NULL", domainvideo.StatusPublished)
}

func (r *Repository) baseHotPageQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).
		Table("video AS v").
		Select("v.id AS video_id, ("+hotScoreExpression+") AS hot_score, v.published_at").
		Joins("LEFT JOIN video_stat AS vs ON vs.video_id = v.id").
		Where("v.status = ? AND v.published_at IS NOT NULL", domainvideo.StatusPublished)
}

func feedPageItemsFromModels(models []domainfeed.FeedPageItem) []*domainfeed.FeedPageItem {
	items := make([]*domainfeed.FeedPageItem, 0, len(models))
	for index := range models {
		items = append(items, &models[index])
	}
	return items
}
