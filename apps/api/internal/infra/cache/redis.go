package infracache

import (
	infraconfig "GCFeed/internal/infra/config"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedisClient 创建 Redis 客户端，连接在首次命令执行时建立。
func NewRedisClient(cfg infraconfig.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  200 * time.Millisecond,
		ReadTimeout:  100 * time.Millisecond,
		WriteTimeout: 100 * time.Millisecond,
		PoolTimeout:  200 * time.Millisecond,
	})
}
