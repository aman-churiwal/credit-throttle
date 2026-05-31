package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	DatabaseURL        string
	RedisURL           string
	TransactionSvcPort string
	RepaymentSvcPort   string
}

func Load() (*Config, error) {
	var missing []string

	databaseURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		missing = append(missing, "DATABASE_URL")
	}

	redisURL, ok := os.LookupEnv("REDIS_URL")
	if !ok {
		missing = append(missing, "REDIS_URL")
	}
	transactionSvcPort, ok := os.LookupEnv("TRANSACTION_SVC_PORT")
	if !ok {
		missing = append(missing, "TRANSACTION_SVC_PORT")
	}
	repaymentSvcPort, ok := os.LookupEnv("REPAYMENT_SVC_PORT")
	if !ok {
		missing = append(missing, "REPAYMENT_SVC_PORT")
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing environment variables: %s", strings.Join(missing, ", "))
	}

	return &Config{
		DatabaseURL:        databaseURL,
		RedisURL:           redisURL,
		TransactionSvcPort: transactionSvcPort,
		RepaymentSvcPort:   repaymentSvcPort,
	}, nil
}
