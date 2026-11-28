package redis

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"chaimir/internal/platform/config"
	"github.com/alicebob/miniredis/v2"
	miniserver "github.com/alicebob/miniredis/v2/server"
	goredis "github.com/redis/go-redis/v9"
)

type leaseReplyHook struct {
	process func(context.Context, goredis.Cmder, goredis.ProcessHook) error
}

func (h leaseReplyHook) DialHook(next goredis.DialHook) goredis.DialHook { return next }
func (h leaseReplyHook) ProcessPipelineHook(next goredis.ProcessPipelineHook) goredis.ProcessPipelineHook {
	return next
}
func (h leaseReplyHook) ProcessHook(next goredis.ProcessHook) goredis.ProcessHook {
	return func(ctx context.Context, cmd goredis.Cmder) error { return h.process(ctx, cmd, next) }
}

func TestLeaseRejectsLateConfirmation(t *testing.T) {
	for _, confirmation := range []int32{1, 2} {
		t.Run(strconv.Itoa(int(confirmation)), func(t *testing.T) {
			server := miniredis.RunT(t)
			client := leaseClient(t, server)
			ttl := 90 * time.Millisecond
			var confirmations atomic.Int32
			client.rdb.AddHook(leaseReplyHook{process: func(ctx context.Context, cmd goredis.Cmder, next goredis.ProcessHook) error {
				err := next(ctx, cmd)
				if err == nil && strings.HasPrefix(cmd.Name(), "eval") && confirmations.Add(1) == confirmation {
					// Redis 已执行成功，响应在传输途中超过确认窗口；此时另一持有者已接管。
					server.FastForward(ttl * 2)
					server.Set("execution", "successor-token")
					server.SetTTL("execution", time.Second)
					time.Sleep(ttl * 2)
				}
				return err
			}})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			called := false
			executionErr := errors.New("execution interrupted")
			err := client.WithLease(ctx, "execution", ttl, func(ctx context.Context) error {
				called = true
				if confirmation == 1 {
					return nil
				}
				<-ctx.Done()
				return errors.Join(executionErr, context.Cause(ctx))
			})
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Errorf("late confirmation extended execution authority: called=%v err=%v", called, err)
			}
			if confirmation == 1 && called {
				t.Error("execution started after initial lease confirmation expired")
			}
			if confirmation == 2 && !errors.Is(err, executionErr) {
				t.Errorf("renewal lost execution error: %v", err)
			}
			value, getErr := server.Get("execution")
			if getErr != nil || value != "successor-token" {
				t.Fatalf("late owner damaged successor: value=%q error=%v", value, getErr)
			}
		})
	}
}

func TestLeaseRenewsWithinRemainingInitialWindow(t *testing.T) {
	server := miniredis.RunT(t)
	client := leaseClient(t, server)
	ttl := 600 * time.Millisecond
	var confirmations atomic.Int32
	var initialDeadline time.Time
	renewed := make(chan struct{}, 1)
	client.rdb.AddHook(leaseReplyHook{process: func(ctx context.Context, cmd goredis.Cmder, next goredis.ProcessHook) error {
		started := time.Now()
		err := next(ctx, cmd)
		if err != nil || !strings.HasPrefix(cmd.Name(), "eval") {
			return err
		}
		switch confirmations.Add(1) {
		case 1:
			initialDeadline = started.Add(ttl)
			time.Sleep(ttl * 2 / 3)
		case 2:
			deadline, ok := ctx.Deadline()
			if !ok || deadline.After(initialDeadline) || !started.Before(initialDeadline) {
				t.Errorf("first renewal restarted the acquisition window: started=%v deadline=%v initial=%v", started, deadline, initialDeadline)
			}
			renewed <- struct{}{}
		}
		return nil
	}})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.WithLease(ctx, "execution", ttl, func(ctx context.Context) error {
		select {
		case <-renewed:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}); err != nil {
		t.Fatal(err)
	}
}

func TestLeaseAcquisitionUsesBoundedNativeIO(t *testing.T) {
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
	warmup, err := client.ObtainLease(context.Background(), "warmup", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := warmup.Release(context.Background()); err != nil {
		t.Fatal(err)
	}
	server.Server().SetPreHook(func(_ *miniserver.Peer, command string, args ...string) bool {
		if strings.HasPrefix(strings.ToLower(command), "eval") && len(args) > 2 && args[2] == "execution" {
			time.Sleep(500 * time.Millisecond)
		}
		return false
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	started := time.Now()
	called := false
	err = client.WithLease(ctx, "execution", 100*time.Millisecond, func(context.Context) error { called = true; return nil })
	if err == nil || called || time.Since(started) >= 400*time.Millisecond {
		t.Fatalf("native acquisition exceeded lease budget: called=%v elapsed=%v error=%v", called, time.Since(started), err)
	}
}

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

func TestQuotaRefundOnlyRemovesItsOwnReservation(t *testing.T) {
	server := miniredis.RunT(t)
	client := leaseClient(t, server)
	ctx := context.Background()
	reserve := func(token string, want bool) {
		t.Helper()
		if ok, err := client.ReserveQuota(ctx, "daily", token, 2, time.Second); err != nil || ok != want {
			t.Fatalf("reserve %q = %t, %v; want %t", token, ok, err, want)
		}
	}
	refund := func(token string) {
		t.Helper()
		if err := client.RefundQuota(ctx, "daily", token); err != nil {
			t.Fatal(err)
		}
	}
	reserve("first", true)
	reserve("first", true)
	reserve("second", true)
	reserve("rejected", false)
	refund("rejected")
	reserve("third", false)
	refund("first")
	refund("first")
	reserve("third", true)
	reserve("fourth", false)
	server.FastForward(2 * time.Second)
	refund("second")
	if server.Exists("daily") {
		t.Fatal("refund recreated an expired quota window")
	}
	reserve("fourth", true)
	reserve("fifth", true)
	refund("third")
	reserve("sixth", false)
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
