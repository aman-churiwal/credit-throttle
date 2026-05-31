package main

import (
	"fmt"
	"os"

	"github.com/aman-churiwal/credit-throttle/shared/config"
)

func main() {
	c, err := config.Load()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println(c.DatabaseURL, c.RedisURL, c.TransactionSvcPort)
	fmt.Println("Successfully started transaction service")
}
