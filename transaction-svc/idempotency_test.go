package main

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func setupTestIdempotency(t *testing.T) (*Idempotency, *redis.Client) {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	return NewIdempotency(rdb), rdb
}

func uniqueTestKey(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())
}

func TestSetThenGetReturnsSameResult(t *testing.T) {
	idem, rdb := setupTestIdempotency(t)
	ctx := context.Background()

	key := uniqueTestKey(t)
	redisKey := "idempotency:" + key

	defer func() {
		_ = rdb.Del(ctx, redisKey).Err()
	}()

	expected := CachedResult{
		StatusCode: 200,
		Body:       []byte("success"),
	}

	if err := idem.Set(ctx, key, expected); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	actual, err := idem.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if actual == nil {
		t.Fatal("expected cached result, got nil")
	}

	if actual.StatusCode != expected.StatusCode {
		t.Fatalf(
			"expected status code %d, got %d",
			expected.StatusCode,
			actual.StatusCode,
		)
	}

	if string(actual.Body) != string(expected.Body) {
		t.Fatalf(
			"expected body %q, got %q",
			string(expected.Body),
			string(actual.Body),
		)
	}
}

func TestGetUnknownKeyReturnsNilNil(t *testing.T) {
	idem, _ := setupTestIdempotency(t)
	ctx := context.Background()

	result, err := idem.Get(ctx, uniqueTestKey(t))

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if result != nil {
		t.Fatalf("expected nil result, got %+v", result)
	}
}

func TestSetStoresKeyWith24HourTTL(t *testing.T) {
	idem, rdb := setupTestIdempotency(t)
	ctx := context.Background()

	key := uniqueTestKey(t)
	redisKey := "idempotency:" + key

	defer func() {
		_ = rdb.Del(ctx, redisKey).Err()
	}()

	err := idem.Set(ctx, key, CachedResult{
		StatusCode: 200,
		Body:       []byte("ok"),
	})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	ttl, err := rdb.TTL(ctx, redisKey).Result()
	if err != nil {
		t.Fatalf("TTL lookup failed: %v", err)
	}

	if ttl <= 0 {
		t.Fatalf("expected positive TTL, got %v", ttl)
	}

	expectedTTL := 24 * time.Hour

	// Allow a few seconds of drift between Set() and TTL().
	if ttl < expectedTTL-5*time.Second || ttl > expectedTTL {
		t.Fatalf(
			"expected TTL close to %v, got %v",
			expectedTTL,
			ttl,
		)
	}
}

func TestSecondSetDoesNotOverwriteFirst(t *testing.T) {
	idem, rdb := setupTestIdempotency(t)
	ctx := context.Background()

	key := uniqueTestKey(t)
	redisKey := "idempotency:" + key

	defer func() {
		_ = rdb.Del(ctx, redisKey).Err()
	}()

	first := CachedResult{
		StatusCode: 200,
		Body:       []byte("first"),
	}

	second := CachedResult{
		StatusCode: 201,
		Body:       []byte("second"),
	}

	if err := idem.Set(ctx, key, first); err != nil {
		t.Fatalf("first Set failed: %v", err)
	}

	if err := idem.Set(ctx, key, second); err != nil && !errors.Is(err, ErrAlreadyCached) {
		t.Fatalf("second Set failed with unexpected error: %v", err)
	}

	result, err := idem.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if result == nil {
		t.Fatal("expected cached result, got nil")
	}

	if result.StatusCode != first.StatusCode {
		t.Fatalf(
			"expected status code %d, got %d",
			first.StatusCode,
			result.StatusCode,
		)
	}

	if string(result.Body) != string(first.Body) {
		t.Fatalf(
			"expected body %q, got %q",
			string(first.Body),
			string(result.Body),
		)
	}
}
