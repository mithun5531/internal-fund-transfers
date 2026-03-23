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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAccountService struct {
	createErr error
	getResp   *dto.AccountResponse
	getErr    error
}

func (s *stubAccountService) Create(_ context.Context, _ dto.CreateAccountRequest) error {
	return s.createErr
}

func (s *stubAccountService) GetByID(_ context.Context, _ int64) (*dto.AccountResponse, error) {
	return s.getResp, s.getErr
}

func setupAccountRouter(h *handler.AccountHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/accounts", h.Create)
	r.GET("/accounts/:account_id", h.GetByID)
	return r
}

func TestCreateAccount_Success(t *testing.T) {
	svc := &stubAccountService{}
	h := handler.NewAccountHandler(svc)
	router := setupAccountRouter(h)

	body := `{"account_id": 1, "initial_balance": "100.50"}`
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestCreateAccount_DuplicateReturns409(t *testing.T) {
	svc := &stubAccountService{createErr: apperror.ErrAccountExists}
	h := handler.NewAccountHandler(svc)
	router := setupAccountRouter(h)

	body := `{"account_id": 1, "initial_balance": "100.50"}`
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)

	var resp dto.ErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "account already exists", resp.Error)
}

func TestCreateAccount_NegativeBalanceReturns422(t *testing.T) {
	svc := &stubAccountService{createErr: apperror.ErrNegativeBalance}
	h := handler.NewAccountHandler(svc)
	router := setupAccountRouter(h)

	body := `{"account_id": 1, "initial_balance": "-10"}`
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestCreateAccount_InvalidBodyReturns400(t *testing.T) {
	svc := &stubAccountService{}
	h := handler.NewAccountHandler(svc)
	router := setupAccountRouter(h)

	body := `{invalid json}`
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAccount_Success(t *testing.T) {
	svc := &stubAccountService{
		getResp: &dto.AccountResponse{AccountID: 1, Balance: "100.50"},
	}
	h := handler.NewAccountHandler(svc)
	router := setupAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/accounts/1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp dto.AccountResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, int64(1), resp.AccountID)
	assert.Equal(t, "100.50", resp.Balance)
}

func TestGetAccount_NotFoundReturns404(t *testing.T) {
	svc := &stubAccountService{getErr: apperror.ErrAccountNotFound}
	h := handler.NewAccountHandler(svc)
	router := setupAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/accounts/999", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetAccount_InvalidIDReturns400(t *testing.T) {
	svc := &stubAccountService{}
	h := handler.NewAccountHandler(svc)
	router := setupAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/accounts/abc", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetAccount_NegativeIDReturns400(t *testing.T) {
	svc := &stubAccountService{}
	h := handler.NewAccountHandler(svc)
	router := setupAccountRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/accounts/-1", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
