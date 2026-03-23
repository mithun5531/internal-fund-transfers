package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mithunp/internal-fund-transfers/internal/config"
	"github.com/mithunp/internal-fund-transfers/internal/database"
	"github.com/mithunp/internal-fund-transfers/internal/handler"
	"github.com/mithunp/internal-fund-transfers/internal/middleware"
	"github.com/mithunp/internal-fund-transfers/internal/model"
	"github.com/mithunp/internal-fund-transfers/internal/repository"
	"github.com/mithunp/internal-fund-transfers/internal/service"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gorm.io/gorm"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load config: %v", err))
	}

	logger := newLogger(cfg.Log.Level)
	defer logger.Sync()

	db, err := database.NewPostgres(cfg.Database, logger)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}

	if err := runMigrations(db); err != nil {
		logger.Fatal("failed to run migrations", zap.Error(err))
	}

	if err := addCheckConstraints(db); err != nil {
		logger.Fatal("failed to add check constraints", zap.Error(err))
	}

	accountRepo := repository.NewAccountRepository(db)
	transactionRepo := repository.NewTransactionRepository(db)
	idempotencyRepo := repository.NewIdempotencyRepository(db)

	accountSvc := service.NewAccountService(accountRepo, logger)
	transferSvc := service.NewTransferService(
		db, accountRepo, transactionRepo, idempotencyRepo,
		cfg.Transfer.MaxRetries, logger,
	)

	accountHandler := handler.NewAccountHandler(accountSvc)
	transactionHandler := handler.NewTransactionHandler(transferSvc)
	healthHandler := handler.NewHealthHandler(db)

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.RequestID())
	router.Use(middleware.Logging(logger))

	router.GET("/health", healthHandler.Check)
	router.POST("/accounts", accountHandler.Create)
	router.GET("/accounts/:account_id", accountHandler.GetByID)
	router.POST("/transactions", transactionHandler.Create)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go startIdempotencyCleanup(ctx, idempotencyRepo, logger)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("server starting", zap.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutting down server", zap.String("signal", sig.String()))
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server forced shutdown", zap.Error(err))
	}

	sqlDB, err := db.DB()
	if err == nil {
		sqlDB.Close()
	}

	logger.Info("server stopped")
}

func newLogger(level string) *zap.Logger {
	var lvl zapcore.Level
	switch level {
	case "debug":
		lvl = zapcore.DebugLevel
	case "warn":
		lvl = zapcore.WarnLevel
	case "error":
		lvl = zapcore.ErrorLevel
	default:
		lvl = zapcore.InfoLevel
	}

	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(lvl),
		Development:      false,
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Sprintf("failed to create logger: %v", err))
	}

	return logger
}

func runMigrations(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.Account{},
		&model.Transaction{},
		&model.IdempotencyKey{},
	)
}

func addCheckConstraints(db *gorm.DB) error {
	constraints := []string{
		`DO $$ BEGIN
			ALTER TABLE accounts ADD CONSTRAINT chk_balance_non_negative CHECK (balance >= 0);
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$`,
		`DO $$ BEGIN
			ALTER TABLE transactions ADD CONSTRAINT chk_amount_positive CHECK (amount > 0);
		EXCEPTION WHEN duplicate_object THEN NULL;
		END $$`,
	}

	for _, sql := range constraints {
		if err := db.Exec(sql).Error; err != nil {
			return err
		}
	}
	return nil
}

func startIdempotencyCleanup(ctx context.Context, repo repository.IdempotencyRepository, logger *zap.Logger) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			deleted, err := repo.DeleteExpired(ctx)
			if err != nil {
				logger.Error("idempotency cleanup failed", zap.Error(err))
			} else if deleted > 0 {
				logger.Info("idempotency keys cleaned up", zap.Int64("deleted", deleted))
			}
		}
	}
}
