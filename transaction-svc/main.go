package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/aman-churiwal/credit-throttle/shared/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	c, err := config.Load()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Config loaded successfully")

	pool, err := pgxpool.New(context.Background(), c.DatabaseURL)
	if err != nil {
		fmt.Println("Unable to connect to database:", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		fmt.Println("Unable to reach database:", err)
		os.Exit(1)
	}

	redisOptions, err := redis.ParseURL(c.RedisURL)
	if redisOptions == nil || err != nil {
		fmt.Println("Unable to parse redis URL:", err)
		os.Exit(1)
	}

	cache := redis.NewClient(redisOptions)
	defer cache.Close()

	if err := cache.Ping(context.Background()).Err(); err != nil {
		fmt.Println("Unable to reach redis:", err)
		os.Exit(1)
	}

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	err = http.ListenAndServe(":"+c.TransactionSvcPort, nil)
	if err != nil {
		fmt.Println("Unable to start the server:", err)
		os.Exit(1)
	}
}
