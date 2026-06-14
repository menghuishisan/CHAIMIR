// redis 封装 Redis 客户端(缓存/会话/限频/计数)。
package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"chaimir/internal/platform/config"

	goredis "github.com/redis/go-redis/v9"
)

// Client 封装 go-redis 客户端。
type Client struct {
	rdb *goredis.Client
}

// New 创建 Redis 客户端并用配置化超时 Ping,缓存依赖不可用时启动失败。
func New(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
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
func (c *Client) Raw() *goredis.Client { return c.rdb }

// Ping 供 HTTP 就绪探针复用同一 Redis 连接检查。
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("Redis 客户端未初始化")
	}
	return c.rdb.Ping(ctx).Err()
}

// Close 释放 Redis 连接资源,由进程退出流程统一调用。
func (c *Client) Close() error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Close()
}

// SetNX 设置键(不存在才设),用于限频窗口;返回是否设置成功。
func (c *Client) SetNX(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, fmt.Errorf("Redis 客户端未初始化")
	}
	ok, err := c.rdb.SetNX(ctx, key, 1, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("Redis SetNX 失败: %w", err)
	}
	return ok, nil
}

// IncrWithTTL 自增计数并在首次设置过期窗口;返回当前计数。
func (c *Client) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, fmt.Errorf("Redis 客户端未初始化")
	}
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

// Decr 自减计数,用于外部动作失败后回滚已占用的限额窗口。
func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, fmt.Errorf("Redis 客户端未初始化")
	}
	n, err := c.rdb.Decr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("Redis Decr 失败: %w", err)
	}
	return n, nil
}

// GetInt64 读取整数缓存值,并显式区分缓存缺失与 Redis 错误。
func (c *Client) GetInt64(ctx context.Context, key string) (int64, bool, error) {
	if c == nil || c.rdb == nil {
		return 0, false, fmt.Errorf("Redis 客户端未初始化")
	}
	raw, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("Redis Get 失败: %w", err)
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("Redis 缓存值不是 int64: %w", err)
	}
	return value, true, nil
}

// SetInt64 写入整数缓存值并设置过期时间。
func (c *Client) SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("Redis 客户端未初始化")
	}
	if err := c.rdb.Set(ctx, key, strconv.FormatInt(value, 10), ttl).Err(); err != nil {
		return fmt.Errorf("Redis Set 失败: %w", err)
	}
	return nil
}

// Delete 删除缓存键,用于权威状态变更后主动失效。
func (c *Client) Delete(ctx context.Context, key string) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("Redis 客户端未初始化")
	}
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("Redis Del 失败: %w", err)
	}
	return nil
}
