package internal_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mithunp/internal-fund-transfers/internal/database"
	"github.com/mithunp/internal-fund-transfers/internal/dto"
	"github.com/mithunp/internal-fund-transfers/internal/handler"
	"github.com/mithunp/internal-fund-transfers/internal/model"
	"github.com/mithunp/internal-fund-transfers/internal/repository"
	"github.com/mithunp/internal-fund-transfers/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/mithunp/internal-fund-transfers/internal/config"
)

func skipIfNoPostgres(t *testing.T) {
	t.Helper()
	if os.Getenv("INTEGRATION_TEST") != "true" {
		t.Skip("skipping integration test; set INTEGRATION_TEST=true and ensure Postgres is running")
	}
}

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	cfg := config.DatabaseConfig{
		Host:     envOrDefault("DB_HOST", "localhost"),
		Port:     5432,
		User:     envOrDefault("DB_USER", "postgres"),
		Password: envOrDefault("DB_PASSWORD", "postgres"),
		Name:     envOrDefault("DB_NAME", "transfers_test"),
		SSLMode:  "disable",
		MaxOpenConns: 50,
		MaxIdleConns: 25,
	}

	logger := zap.NewNop()
	db, err := database.NewPostgres(cfg, logger)
	require.NoError(t, err)

	// Clean slate
	db.Exec("DROP TABLE IF EXISTS transactions")
	db.Exec("DROP TABLE IF EXISTS idempotency_keys")
	db.Exec("DROP TABLE IF EXISTS accounts")

	require.NoError(t, db.AutoMigrate(&model.Account{}, &model.Transaction{}, &model.IdempotencyKey{}))

	db.Exec(`DO $$ BEGIN
		ALTER TABLE accounts ADD CONSTRAINT chk_balance_non_negative CHECK (balance >= 0);
	EXCEPTION WHEN duplicate_object THEN NULL;
	END $$`)
	db.Exec(`DO $$ BEGIN
		ALTER TABLE transactions ADD CONSTRAINT chk_amount_positive CHECK (amount > 0);
	EXCEPTION WHEN duplicate_object THEN NULL;
	END $$`)

	accountRepo := repository.NewAccountRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	idempotencyRepo := repository.NewIdempotencyRepository(db)

	accountSvc := service.NewAccountService(accountRepo, logger)
	transferSvc := service.NewTransferService(db, accountRepo, transactionRepo, idempotencyRepo, 3, logger)

	accountHandler := handler.NewAccountHandler(accountSvc)
	transactionHandler := handler.NewTransactionHandler(transferSvc)
	healthHandler := handler.NewHealthHandler(db)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/health", healthHandler.Check)
	router.POST("/accounts", accountHandler.Create)
	router.GET("/accounts/:account_id", accountHandler.GetByID)
	router.POST("/transactions", transactionHandler.Create)

	return httptest.NewServer(router)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func createAccount(t *testing.T, server *httptest.Server, id int64, balance string) {
	t.Helper()
	body, _ := json.Marshal(dto.CreateAccountRequest{
		AccountID:      id,
		InitialBalance: balance,
	})
	resp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode, "failed to create account %d", id)
}

func getBalance(t *testing.T, server *httptest.Server, id int64) decimal.Decimal {
	t.Helper()
	resp, err := http.Get(fmt.Sprintf("%s/accounts/%d", server.URL, id))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var acct dto.AccountResponse
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&acct))
	d, err := decimal.NewFromString(acct.Balance)
	require.NoError(t, err)
	return d
}

func transfer(t *testing.T, server *httptest.Server, src, dst int64, amount string, idempotencyKey string) int {
	t.Helper()
	body, _ := json.Marshal(dto.TransferRequest{
		SourceAccountID:      src,
		DestinationAccountID: dst,
		Amount:               amount,
	})
	req, _ := http.NewRequest(http.MethodPost, server.URL+"/transactions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

// --- Integration Tests ---

func TestIntegration_HealthCheck(t *testing.T) {
	skipIfNoPostgres(t)
	server := setupTestServer(t)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestIntegration_AccountLifecycle(t *testing.T) {
	skipIfNoPostgres(t)
	server := setupTestServer(t)
	defer server.Close()

	createAccount(t, server, 100, "500.50")

	balance := getBalance(t, server, 100)
	assert.True(t, balance.Equal(decimal.RequireFromString("500.50")))

	// Duplicate → 409
	body, _ := json.Marshal(dto.CreateAccountRequest{AccountID: 100, InitialBalance: "100"})
	resp, err := http.Post(server.URL+"/accounts", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

func TestIntegration_TransferHappyPath(t *testing.T) {
	skipIfNoPostgres(t)
	server := setupTestServer(t)
	defer server.Close()

	createAccount(t, server, 1, "1000.00")
	createAccount(t, server, 2, "500.00")

	status := transfer(t, server, 1, 2, "250.00", "")
	assert.Equal(t, http.StatusCreated, status)

	b1 := getBalance(t, server, 1)
	b2 := getBalance(t, server, 2)
	assert.True(t, b1.Equal(decimal.RequireFromString("750.00")))
	assert.True(t, b2.Equal(decimal.RequireFromString("750.00")))
}

func TestIntegration_TransferInsufficientFunds(t *testing.T) {
	skipIfNoPostgres(t)
	server := setupTestServer(t)
	defer server.Close()

	createAccount(t, server, 1, "100.00")
	createAccount(t, server, 2, "0.00")

	status := transfer(t, server, 1, 2, "200.00", "")
	assert.Equal(t, http.StatusUnprocessableEntity, status)

	// Balance unchanged
	b1 := getBalance(t, server, 1)
	assert.True(t, b1.Equal(decimal.RequireFromString("100.00")))
}

func TestIntegration_TransferPrecision(t *testing.T) {
	skipIfNoPostgres(t)
	server := setupTestServer(t)
	defer server.Close()

	createAccount(t, server, 1, "100.00000")
	createAccount(t, server, 2, "0.00000")

	status := transfer(t, server, 1, 2, "0.00001", "")
	assert.Equal(t, http.StatusCreated, status)

	b1 := getBalance(t, server, 1)
	b2 := getBalance(t, server, 2)

	assert.True(t, b1.Equal(decimal.RequireFromString("99.99999")))
	assert.True(t, b2.Equal(decimal.RequireFromString("0.00001")))
}

func TestIntegration_Idempotency(t *testing.T) {
	skipIfNoPostgres(t)
	server := setupTestServer(t)
	defer server.Close()

	createAccount(t, server, 1, "1000.00")
	createAccount(t, server, 2, "0.00")

	// First request
	status1 := transfer(t, server, 1, 2, "100.00", "idem-key-001")
	assert.Equal(t, http.StatusCreated, status1)

	// Replay with same key — should be 200 (not 201), balance unchanged
	status2 := transfer(t, server, 1, 2, "100.00", "idem-key-001")
	assert.Equal(t, http.StatusOK, status2)

	b1 := getBalance(t, server, 1)
	b2 := getBalance(t, server, 2)
	assert.True(t, b1.Equal(decimal.RequireFromString("900.00")), "expected 900, got %s", b1)
	assert.True(t, b2.Equal(decimal.RequireFromString("100.00")), "expected 100, got %s", b2)
}

// TestIntegration_ConcurrentTransfersFromSameAccount verifies that concurrent
// transfers from the same source account maintain correct balances and never
// allow the balance to go negative.
func TestIntegration_ConcurrentTransfersFromSameAccount(t *testing.T) {
	skipIfNoPostgres(t)
	server := setupTestServer(t)
	defer server.Close()

	createAccount(t, server, 1, "1000.00")
	createAccount(t, server, 2, "0.00")

	concurrency := 100
	amountPerTransfer := "10.00"

	var wg sync.WaitGroup
	successes := make(chan int, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			status := transfer(t, server, 1, 2, amountPerTransfer, "")
			if status == http.StatusCreated {
				successes <- 1
			}
		}()
	}

	wg.Wait()
	close(successes)

	successCount := 0
	for range successes {
		successCount++
	}

	assert.Equal(t, 100, successCount, "all 100 transfers should succeed")

	b1 := getBalance(t, server, 1)
	b2 := getBalance(t, server, 2)

	expectedB1 := decimal.RequireFromString("1000.00").Sub(
		decimal.RequireFromString(amountPerTransfer).Mul(decimal.NewFromInt(int64(successCount))),
	)
	expectedB2 := decimal.RequireFromString(amountPerTransfer).Mul(decimal.NewFromInt(int64(successCount)))

	assert.True(t, b1.Equal(expectedB1), "source balance: expected %s, got %s", expectedB1, b1)
	assert.True(t, b2.Equal(expectedB2), "dest balance: expected %s, got %s", expectedB2, b2)

	// Money conservation
	total := b1.Add(b2)
	assert.True(t, total.Equal(decimal.RequireFromString("1000.00")),
		"money conservation violated: total is %s", total)
}

// TestIntegration_ConcurrentCircularTransfers tests that circular patterns
// (A→B, B→C, C→A) with concurrent goroutines don't deadlock.
func TestIntegration_ConcurrentCircularTransfers(t *testing.T) {
	skipIfNoPostgres(t)
	server := setupTestServer(t)
	defer server.Close()

	createAccount(t, server, 10, "10000.00")
	createAccount(t, server, 20, "10000.00")
	createAccount(t, server, 30, "10000.00")

	concurrency := 50
	var wg sync.WaitGroup

	patterns := [][2]int64{{10, 20}, {20, 30}, {30, 10}}

	for i := 0; i < concurrency; i++ {
		for _, p := range patterns {
			wg.Add(1)
			go func(src, dst int64) {
				defer wg.Done()
				transfer(t, server, src, dst, "1.00", "")
			}(p[0], p[1])
		}
	}

	wg.Wait()

	b10 := getBalance(t, server, 10)
	b20 := getBalance(t, server, 20)
	b30 := getBalance(t, server, 30)

	total := b10.Add(b20).Add(b30)
	assert.True(t, total.Equal(decimal.RequireFromString("30000.00")),
		"money conservation violated in circular test: total is %s", total)

	assert.True(t, b10.GreaterThanOrEqual(decimal.Zero), "account 10 went negative")
	assert.True(t, b20.GreaterThanOrEqual(decimal.Zero), "account 20 went negative")
	assert.True(t, b30.GreaterThanOrEqual(decimal.Zero), "account 30 went negative")
}

// TestIntegration_ConcurrentIdempotency ensures that concurrent requests
// with the same idempotency key result in exactly one execution.
func TestIntegration_ConcurrentIdempotency(t *testing.T) {
	skipIfNoPostgres(t)
	server := setupTestServer(t)
	defer server.Close()

	createAccount(t, server, 1, "1000.00")
	createAccount(t, server, 2, "0.00")

	concurrency := 20
	var wg sync.WaitGroup
	statuses := make([]int, concurrency)

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			statuses[idx] = transfer(t, server, 1, 2, "100.00", "shared-idem-key")
		}(i)
	}

	wg.Wait()

	createdCount := 0
	okCount := 0
	for _, s := range statuses {
		if s == http.StatusCreated {
			createdCount++
		} else if s == http.StatusOK {
			okCount++
		}
	}

	// Exactly one should be 201, rest should be 200
	assert.Equal(t, 1, createdCount, "exactly one request should create the transfer")
	assert.Equal(t, concurrency-1, okCount, "remaining requests should be idempotent replays")

	b1 := getBalance(t, server, 1)
	b2 := getBalance(t, server, 2)
	assert.True(t, b1.Equal(decimal.RequireFromString("900.00")), "expected 900, got %s", b1)
	assert.True(t, b2.Equal(decimal.RequireFromString("100.00")), "expected 100, got %s", b2)
}
