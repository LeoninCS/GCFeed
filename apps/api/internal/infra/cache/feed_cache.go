package infracache

import (
	applicationfeed "GCFeed/internal/application/feed"
	domainfeed "GCFeed/internal/domain/feed"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// FeedCache 使用 Redis 保存 Feed 查询结果。
type FeedCache struct {
	client redis.Cmdable
}

// NewFeedCache 创建 Feed 结果缓存。
func NewFeedCache(client redis.Cmdable) *FeedCache {
	return &FeedCache{client: client}
}

// GetPage 读取缓存中的轻量 Feed 页。
func (c *FeedCache) GetPage(ctx context.Context, key string) (*applicationfeed.FeedPage, bool, error) {
	content, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var page applicationfeed.FeedPage
	if err := json.Unmarshal(content, &page); err != nil {
		return nil, false, err
	}
	return &page, true, nil
}

// SetPage 写入轻量 Feed 页，并设置过期时间。
func (c *FeedCache) SetPage(ctx context.Context, key string, page *applicationfeed.FeedPage, ttl time.Duration) error {
	content, err := json.Marshal(page)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, content, ttl).Err()
}

// GetCards 批量读取视频卡片缓存。
func (c *FeedCache) GetCards(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedCard, error) {
	cards := map[int64]*domainfeed.FeedCard{}
	if len(videoIDs) == 0 {
		return cards, nil
	}

	values, err := c.client.MGet(ctx, cacheKeys(videoIDs, feedCardKey)...).Result()
	if err != nil {
		return nil, err
	}
	for index, value := range values {
		content, ok := cacheValueBytes(value)
		if !ok {
			continue
		}
		var card domainfeed.FeedCard
		if err := json.Unmarshal(content, &card); err != nil {
			continue
		}
		if card.VideoID <= 0 {
			card.VideoID = videoIDs[index]
		}
		cards[card.VideoID] = &card
	}
	return cards, nil
}

// SetCards 批量写入视频卡片缓存。
func (c *FeedCache) SetCards(ctx context.Context, cards map[int64]*domainfeed.FeedCard, ttl time.Duration) error {
	for _, card := range cards {
		if card == nil || card.VideoID <= 0 {
			continue
		}
		content, err := json.Marshal(card)
		if err != nil {
			return err
		}
		if err := c.client.Set(ctx, feedCardKey(card.VideoID), content, ttl).Err(); err != nil {
			return err
		}
	}
	return nil
}

// GetStats 批量读取视频计数缓存。
func (c *FeedCache) GetStats(ctx context.Context, videoIDs []int64) (map[int64]*domainfeed.FeedStat, error) {
	stats := map[int64]*domainfeed.FeedStat{}
	if len(videoIDs) == 0 {
		return stats, nil
	}

	values, err := c.client.MGet(ctx, cacheKeys(videoIDs, feedStatKey)...).Result()
	if err != nil {
		return nil, err
	}
	for index, value := range values {
		content, ok := cacheValueBytes(value)
		if !ok {
			continue
		}
		var stat domainfeed.FeedStat
		if err := json.Unmarshal(content, &stat); err != nil {
			continue
		}
		if stat.VideoID <= 0 {
			stat.VideoID = videoIDs[index]
		}
		stats[stat.VideoID] = &stat
	}
	return stats, nil
}

// SetStats 批量写入视频计数缓存。
func (c *FeedCache) SetStats(ctx context.Context, stats map[int64]*domainfeed.FeedStat, ttl time.Duration) error {
	for _, stat := range stats {
		if stat == nil || stat.VideoID <= 0 {
			continue
		}
		content, err := json.Marshal(stat)
		if err != nil {
			return err
		}
		if err := c.client.Set(ctx, feedStatKey(stat.VideoID), content, ttl).Err(); err != nil {
			return err
		}
	}
	return nil
}

func cacheKeys(videoIDs []int64, build func(int64) string) []string {
	keys := make([]string, 0, len(videoIDs))
	for _, videoID := range videoIDs {
		keys = append(keys, build(videoID))
	}
	return keys
}

func feedCardKey(videoID int64) string {
	return fmt.Sprintf("video:card:v1:%d", videoID)
}

func feedStatKey(videoID int64) string {
	return fmt.Sprintf("video:stat:v1:%d", videoID)
}

func cacheValueBytes(value any) ([]byte, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, false
	case string:
		return []byte(typed), true
	case []byte:
		return typed, true
	default:
		return nil, false
	}
}
