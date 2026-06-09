// redis 测试通用限频与计数辅助不会隐藏错误。
package redis

import (
	"context"
	"testing"
	"time"

	"chaimir/internal/platform/config"
)

// TestNewFailsOnUnavailableRedis 确认 Redis 不可达时启动直接失败,不做静默降级。
func TestNewFailsOnUnavailableRedis(t *testing.T) {
	_, err := New(context.Background(), config.RedisConfig{
		Host:               "127.0.0.1",
		Port:               1,
		DB:                 0,
		PingTimeoutSeconds: 1,
	})
	if err == nil {
		t.Fatalf("expected unavailable redis to fail")
	}
}

// TestHelpersRequireClient 确认空客户端不会让调用方误以为可正常工作。
func TestHelpersRequireClient(t *testing.T) {
	c := &Client{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := c.Ping(ctx); err == nil {
		t.Fatalf("nil redis client ping should fail")
	}
}
