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

var incrWithTTLScript = goredis.NewScript(`
local n = redis.call("INCR", KEYS[1])
if n == 1 then
	redis.call("PEXPIRE", KEYS[1], ARGV[1])
end
return n
`)

// New 创建 Redis 客户端并用配置化超时 Ping,缓存依赖不可用时启动失败。
func New(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	if cfg.PingTimeoutSeconds <= 0 {
		return nil, fmt.Errorf("REDIS_PING_TIMEOUT_SECONDS 必须大于 0")
	}
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	pingCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.PingTimeoutSeconds)*time.Second)
	defer cancel()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("redis 连通性检查失败: %w", err)
	}
	return &Client{rdb: rdb}, nil
}

// Ping 供 HTTP 就绪探针复用同一 Redis 连接检查。
func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("redis 客户端未初始化")
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
		return false, fmt.Errorf("redis 客户端未初始化")
	}
	if key == "" || ttl <= 0 {
		return false, fmt.Errorf("redis SetNX 参数非法")
	}
	ok, err := c.rdb.SetNX(ctx, key, 1, ttl).Result()
	if err != nil {
		return false, fmt.Errorf("redis SetNX 失败: %w", err)
	}
	return ok, nil
}

// IncrWithTTL 原子自增计数并在首次设置过期窗口;返回当前计数。
func (c *Client) IncrWithTTL(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, fmt.Errorf("redis 客户端未初始化")
	}
	if key == "" || ttl <= 0 {
		return 0, fmt.Errorf("redis IncrWithTTL 参数非法")
	}
	n, err := incrWithTTLScript.Run(ctx, c.rdb, []string{key}, ttl.Milliseconds()).Int64()
	if err != nil {
		return 0, fmt.Errorf("redis IncrWithTTL 失败: %w", err)
	}
	return n, nil
}

// Decr 自减计数,用于外部动作失败后回滚已占用的限额窗口。
func (c *Client) Decr(ctx context.Context, key string) (int64, error) {
	if c == nil || c.rdb == nil {
		return 0, fmt.Errorf("redis 客户端未初始化")
	}
	if key == "" {
		return 0, fmt.Errorf("redis Decr 参数非法")
	}
	n, err := c.rdb.Decr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis Decr 失败: %w", err)
	}
	return n, nil
}

// GetInt64 读取整数缓存值,并显式区分缓存缺失与 Redis 错误。
func (c *Client) GetInt64(ctx context.Context, key string) (int64, bool, error) {
	if c == nil || c.rdb == nil {
		return 0, false, fmt.Errorf("redis 客户端未初始化")
	}
	if key == "" {
		return 0, false, fmt.Errorf("redis Get 参数非法")
	}
	raw, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, goredis.Nil) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("redis Get 失败: %w", err)
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("redis 缓存值不是 int64: %w", err)
	}
	return value, true, nil
}

// SetInt64 写入整数缓存值并设置过期时间。
func (c *Client) SetInt64(ctx context.Context, key string, value int64, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("redis 客户端未初始化")
	}
	if key == "" || ttl <= 0 {
		return fmt.Errorf("redis Set 参数非法")
	}
	if err := c.rdb.Set(ctx, key, strconv.FormatInt(value, 10), ttl).Err(); err != nil {
		return fmt.Errorf("redis Set 失败: %w", err)
	}
	return nil
}

// GetBytes 读取通用字节缓存,显式区分缓存缺失与 Redis 错误。
func (c *Client) GetBytes(ctx context.Context, key string) ([]byte, bool, error) {
	if c == nil || c.rdb == nil {
		return nil, false, fmt.Errorf("redis 客户端未初始化")
	}
	if key == "" {
		return nil, false, fmt.Errorf("redis GetBytes 参数非法")
	}
	raw, err := c.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis GetBytes 失败: %w", err)
	}
	return raw, true, nil
}

// SetBytes 写入通用字节缓存并设置过期时间,用于模块保存已校验的 JSON 等结构化结果。
func (c *Client) SetBytes(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("redis 客户端未初始化")
	}
	if key == "" || ttl <= 0 {
		return fmt.Errorf("redis SetBytes 参数非法")
	}
	if len(value) == 0 {
		return fmt.Errorf("redis SetBytes 值不能为空")
	}
	if err := c.rdb.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis SetBytes 失败: %w", err)
	}
	return nil
}

// Delete 删除缓存键,用于权威状态变更后主动失效。
func (c *Client) Delete(ctx context.Context, key string) error {
	if c == nil || c.rdb == nil {
		return fmt.Errorf("redis 客户端未初始化")
	}
	if key == "" {
		return fmt.Errorf("redis Del 参数非法")
	}
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis Del 失败: %w", err)
	}
	return nil
}
