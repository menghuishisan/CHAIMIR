// Package redis 封装 Redis 客户端(缓存/会话/限频/计数)。
// 依据 docs/总-技术选型.md §4:Redis 用于未读计数、在线状态、限频、队列。
package redis

import (
	"context"
	"fmt"
	"time"

	"chaimir/internal/platform/config"

	"github.com/redis/go-redis/v9"
)

// Client 封装 go-redis 客户端。
type Client struct {
	rdb *redis.Client
}

// New 创建 Redis 客户端并用配置化超时 Ping,缓存依赖不可用时启动失败。
func New(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	pingCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.PingTimeoutSeconds)*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("Redis 连通性检查失败: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

// Raw 暴露底层 go-redis 客户端,用于模块实现队列、限频等 Redis 原语。
func (c *Client) Raw() *redis.Client { return c.rdb }

// Ping 供 HTTP 就绪探针复用同一 Redis 连接检查。
func (c *Client) Ping(ctx context.Context) error { return c.rdb.Ping(ctx).Err() }

// Close 释放 Redis 连接资源,由进程退出流程统一调用。
func (c *Client) Close() error { return c.rdb.Close() }

// SetNX 设置键(不存在才设),用于限频窗口(如同号 60s 一条短信);返回是否设置成功。
func (c *Client) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ok, err := c.rdb.SetNX(ctx, key, 1, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("Redis SetNX 失败: %w", err)
	}
	return ok, nil
}

// IncrWithTTL 自增计数并在首次设置过期窗口(限频日上限用);返回当前计数。
func (c *Client) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	n, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("Redis Incr 失败: %w", err)
	}
	if n == 1 {
		if err := c.rdb.Expire(ctx, key, ttl).Err(); err != nil {
			return n, fmt.Errorf("Redis Expire 失败: %w", err)
		}
	}
	return n, nil
}
