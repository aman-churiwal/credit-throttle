package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool  *pgxpool.Pool
	store *Store
}

type RepayRequest struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
}

type RepayResponse struct {
	AccountID       string `json:"account_id"`
	Amount          int64  `json:"amount"`
	AvailableCredit int64  `json:"available_credit"`
}

func NewHandler(pool *pgxpool.Pool, store *Store) *Handler {
	return &Handler{pool: pool, store: store}
}

func writeJSON(w http.ResponseWriter, status int, body any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	_, err = w.Write(b)
	if err != nil {
		return err
	}

	return nil
}

func (h *Handler) Repay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req RepayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if req.AccountID == "" || req.Amount <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	tx, err := h.pool.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer tx.Rollback(r.Context())

	account, err := h.store.GetAccount(r.Context(), tx, req.AccountID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	newBalance := account.AvailableCredit + req.Amount

	if newBalance > account.CreditLimit {
		newBalance = account.CreditLimit
	}

	_, err = h.store.Repay(r.Context(), tx, req.AccountID, req.Amount, newBalance, account.Version)
	if err != nil {
		if errors.Is(err, ErrStaleRead) {
			maxRetries := 3

			for attempt := 0; attempt < maxRetries; attempt++ {
				tx, err = h.pool.Begin(r.Context())
				if err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}

				updatedAccount, err := h.store.GetAccount(r.Context(), tx, req.AccountID)
				if err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}

				updatedNewBalance := updatedAccount.AvailableCredit + req.Amount

				if updatedNewBalance > updatedAccount.CreditLimit {
					updatedNewBalance = updatedAccount.CreditLimit
				}

				_, err = h.store.Repay(r.Context(), tx, req.AccountID, req.Amount, updatedNewBalance, updatedAccount.Version)
				if errors.Is(err, ErrStaleRead) {
					tx.Rollback(r.Context())
					fmt.Printf("stale read on attempt %d, retrying...\n", attempt+1)
					time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
					continue
				}
				if err != nil {
					tx.Rollback(r.Context())
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
				if err := tx.Commit(r.Context()); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}

				repayResponse := RepayResponse{
					AccountID:       updatedAccount.ID,
					Amount:          req.Amount,
					AvailableCredit: updatedNewBalance,
				}
				writeJSON(w, http.StatusOK, repayResponse)
				return
			}
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, RepayResponse{
		AccountID:       account.ID,
		Amount:          req.Amount,
		AvailableCredit: newBalance,
	})
	return
}
