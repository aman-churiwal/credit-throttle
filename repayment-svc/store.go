package main

import (
	"context"
	"errors"

	"github.com/aman-churiwal/credit-throttle/shared/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrStaleRead = errors.New("stale read — concurrent update detected")

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) GetAccount(ctx context.Context, tx pgx.Tx, accountID string) (*models.Account, error) {
	query := `SELECT id, owner, credit_limit, available_credit, version FROM accounts WHERE id = $1 FOR UPDATE`
	var account models.Account
	row := tx.QueryRow(ctx, query, accountID)

	if err := row.Scan(&account.ID, &account.Owner, &account.CreditLimit,
		&account.AvailableCredit, &account.Version); err != nil {
		return nil, err
	}

	return &account, nil
}

func (s *Store) Repay(ctx context.Context, tx pgx.Tx, accountID string, amount, newBalance, version int64) (*models.Transaction, error) {
	updateQuery := `UPDATE accounts SET available_credit = $1, version = version + 1 WHERE id = $2 AND version = $3`

	result, err := tx.Exec(ctx, updateQuery, newBalance, accountID, version)
	if err != nil {
		return nil, err
	}

	if result.RowsAffected() == 0 {
		return nil, ErrStaleRead
	}

	transaction := models.Transaction{
		AccountID: accountID,
		Amount:    amount,
		TxType:    "repay",
		Status:    "committed",
	}

	transactionInsertQuery := `INSERT INTO transactions (account_id, amount, tx_type, status) VALUES ($1, $2, $3, $4) RETURNING id`

	row := tx.QueryRow(ctx, transactionInsertQuery, accountID, amount, "repay", "committed")
	if err := row.Scan(&transaction.ID); err != nil {
		return nil, err
	}

	auditInsertQuery := `INSERT INTO audit_logs (account_id, tx_id, event_type, amount, balance_after) VALUES ($1, $2, $3, $4, $5)`

	_, err = tx.Exec(ctx, auditInsertQuery, accountID, transaction.ID, "repay", amount, newBalance)

	return &transaction, err
}
