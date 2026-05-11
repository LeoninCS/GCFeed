package infracache

import (
	applicationfeed "GCFeed/internal/application/feed"
	"context"
	"encoding/json"
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

// Get 读取缓存中的 FeedResult。
func (c *FeedCache) Get(ctx context.Context, key string) (*applicationfeed.FeedResult, bool, error) {
	content, err := c.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}

	var result applicationfeed.FeedResult
	if err := json.Unmarshal(content, &result); err != nil {
		return nil, false, err
	}
	return &result, true, nil
}

// Set 写入 FeedResult，并设置过期时间。
func (c *FeedCache) Set(ctx context.Context, key string, result *applicationfeed.FeedResult, ttl time.Duration) error {
	content, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return c.client.Set(ctx, key, content, ttl).Err()
}
