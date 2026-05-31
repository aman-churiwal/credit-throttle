package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var releaseScript = redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)

type Locker struct {
	rdb *redis.Client
}

func NewLocker(rdb *redis.Client) *Locker {
	return &Locker{rdb: rdb}
}

func (l *Locker) Acquire(ctx context.Context, key string, ttl time.Duration) (string, error) {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	ok, err := l.rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return "", err
	}

	if !ok {
		return "", fmt.Errorf("lock already acquired for key: %s", key)
	}

	return token, nil
}

func (l *Locker) Release(ctx context.Context, key, token string) error {
	result, err := releaseScript.Run(ctx, l.rdb, []string{key}, token).Int()

	if err != nil {
		return err
	}

	if result == 0 {
		fmt.Printf("lock not owned by %s or lock does not exist\n", key)
	}

	return nil
}
