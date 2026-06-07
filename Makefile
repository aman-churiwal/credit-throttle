.PHONY: up down test load-test

up:
	docker compose up --build -d

down:
	docker compose down -v

test:
	go test ./... -race -count=1

load-test:
	k6 run load-tests/spend.js
	k6 run load-tests/idempotency.js
	k6 run load-tests/repayment.js