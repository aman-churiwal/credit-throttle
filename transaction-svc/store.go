package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

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

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) GetAccount(ctx context.Context, tx pgx.Tx, accountID string) (*Account, error) {
	query := `SELECT id, owner, credit_limit, available_credit, version FROM accounts WHERE id = $1 FOR UPDATE`
	var account Account
	row := tx.QueryRow(ctx, query, accountID)

	if err := row.Scan(&account.ID, &account.Owner, &account.CreditLimit,
		&account.AvailableCredit, &account.Version); err != nil {
		return nil, err
	}

	return &account, nil
}

func (s *Store) Spend(ctx context.Context, tx pgx.Tx, accountID string, amount, availableCredit int64, idempotencyKey string) (*Transaction, error) {
	updateBalanceQuery := `UPDATE accounts SET available_credit = available_credit - $1, version = version + 1 WHERE id = $2`
	_, err := tx.Exec(ctx, updateBalanceQuery, amount, accountID)
	if err != nil {
		return nil, err
	}

	insertTransactionQuery := `INSERT INTO transactions (account_id, idempotency_key, tx_type, amount, status) VALUES ($1, $2, $3, $4, $5) RETURNING id, account_id, idempotency_key, tx_type, amount, status`
	row := tx.QueryRow(ctx, insertTransactionQuery, accountID, idempotencyKey, "spend", amount, "pending")

	var transaction Transaction
	if err := row.Scan(&transaction.ID, &transaction.AccountID, &transaction.IdempotencyKey, &transaction.TxType, &transaction.Amount, &transaction.Status); err != nil {
		return nil, err
	}

	insertAuditQuery := `INSERT INTO audit_logs (account_id, tx_id, event_type, amount, balance_after) VALUES ($1, $2, $3, $4, $5)`
	_, err = tx.Exec(ctx, insertAuditQuery, accountID, transaction.ID, "spend", amount, availableCredit-amount)
	if err != nil {
		return nil, err
	}

	return &transaction, nil
}
