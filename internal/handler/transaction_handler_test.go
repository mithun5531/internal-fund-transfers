package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mithunp/internal-fund-transfers/internal/apperror"
	"github.com/mithunp/internal-fund-transfers/internal/dto"
	"github.com/mithunp/internal-fund-transfers/internal/handler"
	"github.com/mithunp/internal-fund-transfers/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubTransferService struct {
	result *service.TransferResult
	err    error
}

func (s *stubTransferService) Transfer(_ context.Context, _ string, _ dto.TransferRequest) (*service.TransferResult, error) {
	return s.result, s.err
}

func setupTransactionRouter(h *handler.TransactionHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/transactions", h.Create)
	return r
}

func TestTransfer_Success(t *testing.T) {
	svc := &stubTransferService{result: &service.TransferResult{StatusCode: 201}}
	h := handler.NewTransactionHandler(svc)
	router := setupTransactionRouter(h)

	body := `{"source_account_id": 1, "destination_account_id": 2, "amount": "50.00"}`
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestTransfer_IdempotentReplay(t *testing.T) {
	svc := &stubTransferService{result: &service.TransferResult{StatusCode: 201, Replayed: true}}
	h := handler.NewTransactionHandler(svc)
	router := setupTransactionRouter(h)

	body := `{"source_account_id": 1, "destination_account_id": 2, "amount": "50.00"}`
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", "test-key-1")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestTransfer_InsufficientFundsReturns422(t *testing.T) {
	svc := &stubTransferService{err: apperror.ErrInsufficientFunds}
	h := handler.NewTransactionHandler(svc)
	router := setupTransactionRouter(h)

	body := `{"source_account_id": 1, "destination_account_id": 2, "amount": "99999.00"}`
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var resp dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "insufficient funds", resp.Error)
}

func TestTransfer_SameAccountReturns422(t *testing.T) {
	svc := &stubTransferService{err: apperror.ErrSameAccount}
	h := handler.NewTransactionHandler(svc)
	router := setupTransactionRouter(h)

	body := `{"source_account_id": 1, "destination_account_id": 1, "amount": "10.00"}`
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestTransfer_AccountNotFoundReturns404(t *testing.T) {
	svc := &stubTransferService{err: apperror.ErrAccountNotFound}
	h := handler.NewTransactionHandler(svc)
	router := setupTransactionRouter(h)

	body := `{"source_account_id": 999, "destination_account_id": 2, "amount": "10.00"}`
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestTransfer_InvalidAmountReturns422(t *testing.T) {
	svc := &stubTransferService{err: apperror.ErrInvalidAmount}
	h := handler.NewTransactionHandler(svc)
	router := setupTransactionRouter(h)

	body := `{"source_account_id": 1, "destination_account_id": 2, "amount": "-10.00"}`
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestTransfer_InvalidBodyReturns400(t *testing.T) {
	svc := &stubTransferService{}
	h := handler.NewTransactionHandler(svc)
	router := setupTransactionRouter(h)

	body := `{bad json}`
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
