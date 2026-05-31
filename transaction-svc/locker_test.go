package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func setupTestLocker(t *testing.T) (*Locker, *redis.Client) {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	ctx := context.Background()

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("Unable to connect to redis: %v", err)
	}

	return NewLocker(rdb), rdb
}

func TestAcquireLock(t *testing.T) {
	locker, rdb := setupTestLocker(t)

	ctx := context.Background()
	key := "test-lock-acquire-success"

	defer rdb.Del(ctx, key)

	token, err := locker.Acquire(ctx, key, 5*time.Second)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	val, err := rdb.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("expected key to exist: %v", err)
	}

	if val != token {
		t.Fatalf("expected token %s, got %s", token, val)
	}
}

func TestAcquireAlreadyLocked(t *testing.T) {
	locker, rdb := setupTestLocker(t)

	ctx := context.Background()
	key := "test-lock-already-locked"

	defer rdb.Del(ctx, key)

	_, err := locker.Acquire(ctx, key, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = locker.Acquire(ctx, key, 5*time.Second)

	if err == nil {
		t.Fatal("expected lock acquisition to fail")
	}
}

func TestReleaseSuccess(t *testing.T) {
	locker, rdb := setupTestLocker(t)

	ctx := context.Background()
	key := "test-lock-release-success"

	defer rdb.Del(ctx, key)

	token, err := locker.Acquire(ctx, key, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = locker.Release(ctx, key, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exists, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists != 0 {
		t.Fatal("expected lock to be deleted")
	}
}

func TestReleaseWrongToken(t *testing.T) {
	locker, rdb := setupTestLocker(t)

	ctx := context.Background()
	key := "test-lock-release-wrong-token"

	defer rdb.Del(ctx, key)

	_, err := locker.Acquire(ctx, key, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = locker.Release(ctx, key, "wrong-token")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exists, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if exists != 1 {
		t.Fatal("expected lock to remain")
	}
}

func TestAcquireAfterTTLExpiry(t *testing.T) {
	locker, rdb := setupTestLocker(t)

	ctx := context.Background()
	key := "test-lock-ttl-expiry"

	defer rdb.Del(ctx, key)

	_, err := locker.Acquire(ctx, key, 1*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	_, err = locker.Acquire(ctx, key, 1*time.Second)
	if err != nil {
		t.Fatalf("expected lock acquisition after expiry, got %v", err)
	}
}

func TestAcquireConcurrent(t *testing.T) {
	locker, rdb := setupTestLocker(t)

	ctx := context.Background()
	key := "test-lock-concurrent"

	defer rdb.Del(ctx, key)

	const workers = 10

	var (
		wg            sync.WaitGroup
		mu            sync.Mutex
		successTokens []string
	)

	start := make(chan struct{})

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			token, err := locker.Acquire(ctx, key, 10*time.Second)
			if err != nil {
				return
			}

			mu.Lock()
			successTokens = append(successTokens, token)
			mu.Unlock()
		}()
	}

	close(start)
	wg.Wait()

	if len(successTokens) != 1 {
		t.Fatalf(
			"expected exactly one winner, got %d",
			len(successTokens),
		)
	}

	storedToken, err := rdb.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("failed to read lock from redis: %v", err)
	}

	if storedToken != successTokens[0] {
		t.Fatalf(
			"stored token %s does not match winner token %s",
			storedToken,
			successTokens[0],
		)
	}
}
