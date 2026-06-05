package models

import "time"

type Account struct {
	ID              string
	Owner           string
	CreditLimit     int64
	AvailableCredit int64
	Version         int64
}

type Transaction struct {
	ID             string `json:"id"`
	AccountID      string `json:"account_id"`
	IdempotencyKey string `json:"idempotency_key"`
	TxType         string `json:"tx_type"`
	Amount         int64  `json:"amount"`
	Status         string `json:"status"`
}

type AuditEvent struct {
	ID           string    `json:"id"`
	TxID         string    `json:"tx_id"`
	EventType    string    `json:"event_type"`
	Amount       int64     `json:"amount"`
	BalanceAfter int64     `json:"balance_after"`
	RecordedAt   time.Time `json:"recorded_at"`
}

type AuditResponse struct {
	AccountID string       `json:"account_id"`
	Events    []AuditEvent `json:"events"`
}
