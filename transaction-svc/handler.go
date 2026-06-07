package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aman-churiwal/credit-throttle/shared/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool        *pgxpool.Pool
	store       *Store
	locker      *Locker
	idempotency *Idempotency
}

type SpendRequest struct {
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount"`
}

type SpendResponse struct {
	TransactionID   string `json:"transaction_id"`
	AccountID       string `json:"account_id"`
	Amount          int64  `json:"amount"`
	AvailableCredit int64  `json:"available_credit"`
}

func NewHandler(pool *pgxpool.Pool, store *Store, locker *Locker, idempotency *Idempotency) *Handler {
	return &Handler{pool: pool, store: store, locker: locker, idempotency: idempotency}
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

func (h *Handler) Spend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var req SpendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if req.Amount <= 0 || req.AccountID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	idempotencyKey := r.Header.Get("X-Idempotency-Key")
	if idempotencyKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing X-Idempotency-Key header"})
		return
	}

	cachedTransaction, err := h.idempotency.Get(r.Context(), idempotencyKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if cachedTransaction != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(cachedTransaction.StatusCode)
		w.Write(cachedTransaction.Body)
		return
	}

	lockKey := fmt.Sprintf("lock:account:%s", req.AccountID)
	var token string
	var lockErr error
	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		token, lockErr = h.locker.Acquire(r.Context(), lockKey, 500*time.Millisecond)
		if lockErr == nil {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	if lockErr != nil {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": lockErr.Error()})
		return
	}
	defer h.locker.Release(r.Context(), lockKey, token)

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

	if account.AvailableCredit < req.Amount {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "insufficient funds"})
		return
	}

	spendTransaction, err := h.store.Spend(r.Context(), tx, req.AccountID, req.Amount, account.AvailableCredit, idempotencyKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	spendResponse := SpendResponse{
		TransactionID:   spendTransaction.ID,
		AccountID:       req.AccountID,
		Amount:          spendTransaction.Amount,
		AvailableCredit: account.AvailableCredit - spendTransaction.Amount,
	}

	body, err := json.Marshal(spendResponse)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	transactionResult := CachedResult{
		Body:       body,
		StatusCode: http.StatusCreated,
	}

	if err := h.idempotency.Set(r.Context(), idempotencyKey, transactionResult); err != nil && !errors.Is(err, ErrAlreadyCached) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, spendResponse)
}

func (h *Handler) Audit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing account id"})
		return
	}
	accountID := parts[2]

	auditLogs, err := h.store.GetAuditLogs(r.Context(), accountID, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	auditResponse := models.AuditResponse{
		AccountID: accountID,
		Events:    auditLogs,
	}
	writeJSON(w, http.StatusOK, auditResponse)
}
