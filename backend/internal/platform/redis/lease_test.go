package redis

import (
	"context"
	"errors"
	"net"
	"strconv"
	"testing"
	"time"

	"chaimir/internal/platform/config"
	"github.com/alicebob/miniredis/v2"
	miniserver "github.com/alicebob/miniredis/v2/server"
	goredis "github.com/redis/go-redis/v9"
)

func leaseClient(t *testing.T, server *miniredis.Miniredis) *Client {
	t.Helper()
	client := &Client{rdb: goredis.NewClient(&goredis.Options{Addr: server.Addr(), MaxRetries: -1})}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	return client
}

func TestLeaseExcludesOtherClientsAndReleasesAfterWork(t *testing.T) {
	server := miniredis.RunT(t)
	first, second := leaseClient(t, server), leaseClient(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	entered, finish, result := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		result <- first.WithLease(ctx, "execution", time.Second, func(ctx context.Context) error {
			close(entered)
			select {
			case <-finish:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	called := false
	err := second.WithLease(ctx, "execution", time.Second, func(context.Context) error { called = true; return nil })
	if !errors.Is(err, ErrLeaseHeld) || called {
		t.Fatalf("second owner entered: called=%v error=%v", called, err)
	}
	close(finish)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	if err := second.WithLease(ctx, "execution", time.Second, func(context.Context) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if server.Exists("execution") {
		t.Fatal("lease survived completed cleanup")
	}
}

func TestLeaseKeepsRenewingDuringCanceledCleanup(t *testing.T) {
	server := miniredis.RunT(t)
	first, second := leaseClient(t, server), leaseClient(t, server)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cleanupCancel()
	cleanup, finish, result := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	ttl := 300 * time.Millisecond
	go func() {
		result <- first.WithLease(ctx, "execution", ttl, func(ctx context.Context) error {
			cancel()
			<-ctx.Done()
			close(cleanup)
			select {
			case <-finish:
				return nil
			case <-cleanupCtx.Done():
				return cleanupCtx.Err()
			}
		})
	}()
	select {
	case <-cleanup:
	case <-cleanupCtx.Done():
		t.Fatal(cleanupCtx.Err())
	}
	server.FastForward(ttl / 2)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for server.TTL("execution") < ttl*2/3 {
		select {
		case <-ticker.C:
		case <-cleanupCtx.Done():
			t.Fatal("canceled request stopped renewal before cleanup")
		}
	}
	err := second.WithLease(cleanupCtx, "execution", ttl, func(context.Context) error { return errors.New("entered during cleanup") })
	if !errors.Is(err, ErrLeaseHeld) {
		t.Fatalf("cleanup lost exclusion: %v", err)
	}
	close(finish)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
}

func TestLeaseLossCancelsWorkWithoutDeletingSuccessor(t *testing.T) {
	server := miniredis.RunT(t)
	client := leaseClient(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	entered, result := make(chan struct{}), make(chan error, 1)
	go func() {
		result <- client.WithLease(ctx, "execution", 30*time.Millisecond, func(ctx context.Context) error {
			close(entered)
			<-ctx.Done()
			return context.Cause(ctx)
		})
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	server.Set("execution", "successor-token")
	server.SetTTL("execution", time.Second)
	select {
	case err := <-result:
		if err == nil {
			t.Fatal("lost lease reported success")
		}
	case <-ctx.Done():
		t.Fatal("lost lease did not cancel work")
	}
	value, err := server.Get("execution")
	if err != nil || value != "successor-token" {
		t.Fatalf("old owner damaged successor lease: value=%q error=%v", value, err)
	}
}

func TestLeasePreservesCallbackErrorAndReleasesOnPanic(t *testing.T) {
	server := miniredis.RunT(t)
	client := leaseClient(t, server)
	want := errors.New("execution failed")
	if err := client.WithLease(context.Background(), "execution", time.Second, func(context.Context) error { return want }); !errors.Is(err, want) {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() != want {
				t.Error("panic was swallowed or replaced")
			}
		}()
		err := client.WithLease(context.Background(), "execution", time.Second, func(context.Context) error { panic(want) })
		t.Fatalf("expected callback panic, returned %v", err)
	}()
	if server.Exists("execution") {
		t.Fatal("panic retained execution lease")
	}
}

func TestRedisOperationsHonorContextDeadline(t *testing.T) {
	server := miniredis.RunT(t)
	host, portText, err := net.SplitHostPort(server.Addr())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	client, err := New(context.Background(), config.RedisConfig{Host: host, Port: port, PingTimeoutSeconds: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	server.Server().SetPreHook(func(_ *miniserver.Peer, command string, _ ...string) bool {
		if command == "ping" || command == "PING" {
			time.Sleep(300 * time.Millisecond)
		}
		return false
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = client.Ping(ctx)
	if err == nil || time.Since(started) >= 200*time.Millisecond {
		t.Fatalf("Redis ignored operation deadline: elapsed=%v error=%v", time.Since(started), err)
	}
}
