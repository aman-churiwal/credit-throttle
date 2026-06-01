package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type CachedResult struct {
	StatusCode int
	Body       []byte
}

type Idempotency struct {
	rdb *redis.Client
}

var ErrAlreadyCached = errors.New("result already cached")

func NewIdempotency(rdb *redis.Client) *Idempotency {
	return &Idempotency{rdb: rdb}
}

func (i *Idempotency) Get(ctx context.Context, key string) (*CachedResult, error) {
	formattedKey := fmt.Sprintf("idempotency:%s", key)
	val, err := i.rdb.Get(ctx, formattedKey).Result()

	if errors.Is(err, redis.Nil) {
		fmt.Printf("Cache Miss: Key - %s not found\n", formattedKey)
		return nil, nil
	}

	if err != nil {
		fmt.Printf("Error finding the key - %s: %v\n", formattedKey, err)
		return nil, err
	}

	var result CachedResult
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		fmt.Printf("Error unmarshalling the result for the key - %s: %v\n", formattedKey, err)
		return nil, err
	}

	return &result, nil
}

func (i *Idempotency) Set(ctx context.Context, key string, result CachedResult) error {
	formattedKey := fmt.Sprintf("idempotency:%s", key)

	marshalledResult, err := json.Marshal(result)
	if err != nil {
		fmt.Printf("Error marshalling the result for the key - %s: %v\n", formattedKey, err)
		return err
	}

	ok, err := i.rdb.SetNX(ctx, formattedKey, marshalledResult, 24*time.Hour).Result()
	if err != nil {
		return err
	}
	if !ok {
		return ErrAlreadyCached
	}

	return nil
}
