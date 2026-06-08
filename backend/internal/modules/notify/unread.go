// M10 未读计数:封装 Redis 计数键,服务层不直接拼接缓存实现细节。
package notify

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/platform/redis"
	"chaimir/pkg/apperr"

	goredis "github.com/redis/go-redis/v9"
)

type redisUnreadCounter struct {
	redis *redis.Client
	ttl   time.Duration
}

// newRedisUnreadCounter 构造 Redis 未读计数器。
func newRedisUnreadCounter(client *redis.Client, ttl time.Duration) unreadCounter {
	return &redisUnreadCounter{redis: client, ttl: ttl}
}

// Increment 递增指定租户账号未读数并维持清理窗口。
func (c *redisUnreadCounter) Increment(ctx context.Context, tenantID, accountID int64) (int64, error) {
	if c.redis == nil {
		return 0, apperr.ErrNotifySendFailed
	}
	return c.redis.IncrWithTTL(ctx, unreadKey(tenantID, accountID), c.ttl)
}

// Get 读取未读缓存;缓存 miss 交由服务层按 notification 权威状态重建。
func (c *redisUnreadCounter) Get(ctx context.Context, tenantID int64, accountID int64) (int64, bool, error) {
	if c.redis == nil {
		return 0, false, apperr.ErrNotifySendFailed
	}
	n, err := c.redis.Raw().Get(ctx, unreadKey(tenantID, accountID)).Int64()
	if err != nil {
		if err == goredis.Nil {
			return 0, false, nil
		}
		return 0, false, err
	}
	return n, true, nil
}

// Set 回写未读缓存,让 miss 后的权威值重新进入红点缓存路径。
func (c *redisUnreadCounter) Set(ctx context.Context, tenantID int64, accountID, count int64) error {
	if c.redis == nil {
		return apperr.ErrNotifySendFailed
	}
	return c.redis.Raw().Set(ctx, unreadKey(tenantID, accountID), count, c.ttl).Err()
}

// Reset 清空指定账号未读计数。
func (c *redisUnreadCounter) Reset(ctx context.Context, tenantID int64, accountID int64) error {
	if c.redis == nil {
		return apperr.ErrNotifySendFailed
	}
	return c.redis.Raw().Del(ctx, unreadKey(tenantID, accountID)).Err()
}

// unreadKey 生成租户隔离的未读计数缓存键。
func unreadKey(tenantID, accountID int64) string {
	return fmt.Sprintf("tenant:%d:unread:%d", tenantID, accountID)
}
