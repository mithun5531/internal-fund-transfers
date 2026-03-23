package service

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mithunp/internal-fund-transfers/internal/apperror"
	"github.com/mithunp/internal-fund-transfers/internal/dto"
	"github.com/mithunp/internal-fund-transfers/internal/model"
	"github.com/mithunp/internal-fund-transfers/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TransferResult struct {
	StatusCode int
	Body       interface{}
	Replayed   bool
}

type TransferService interface {
	Transfer(ctx context.Context, idempotencyKey string, req dto.TransferRequest) (*TransferResult, error)
}

type transferService struct {
	db              *gorm.DB
	accountRepo     repository.AccountRepository
	transactionRepo repository.TransactionRepository
	idempotencyRepo repository.IdempotencyRepository
	maxRetries      int
	logger          *zap.Logger
}

func NewTransferService(
	db *gorm.DB,
	accountRepo repository.AccountRepository,
	transactionRepo repository.TransactionRepository,
	idempotencyRepo repository.IdempotencyRepository,
	maxRetries int,
	logger *zap.Logger,
) TransferService {
	return &transferService{
		db:              db,
		accountRepo:     accountRepo,
		transactionRepo: transactionRepo,
		idempotencyRepo: idempotencyRepo,
		maxRetries:      maxRetries,
		logger:          logger,
	}
}

func (s *transferService) Transfer(ctx context.Context, idempotencyKey string, req dto.TransferRequest) (*TransferResult, error) {
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil {
		return nil, apperror.ErrInvalidRequestBody
	}

	if !amount.IsPositive() {
		return nil, apperror.ErrInvalidAmount
	}

	if req.SourceAccountID == req.DestinationAccountID {
		return nil, apperror.ErrSameAccount
	}

	var result *TransferResult
	var lastErr error

	for attempt := 0; attempt <= s.maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if idempotencyKey != "" {
			result, lastErr = s.transferWithIdempotency(ctx, idempotencyKey, req, amount)
		} else {
			result, lastErr = s.executeTransfer(ctx, req, amount)
		}

		if lastErr == nil {
			return result, nil
		}

		if !isRetryableError(lastErr) {
			return nil, lastErr
		}

		s.logger.Warn("retrying transfer due to transient error",
			zap.Int("attempt", attempt+1),
			zap.Error(lastErr),
		)

		base := time.Duration(1<<uint(attempt)) * 10 * time.Millisecond
		jitter := time.Duration(rand.Int63n(int64(base) + 1))
		backoff := base + jitter

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
	}

	s.logger.Error("transfer failed after retries",
		zap.Int("max_retries", s.maxRetries),
		zap.Error(lastErr),
	)

	return nil, lastErr
}

func (s *transferService) transferWithIdempotency(
	ctx context.Context,
	key string,
	req dto.TransferRequest,
	amount decimal.Decimal,
) (*TransferResult, error) {
	var result *TransferResult

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		existing, err := s.idempotencyRepo.FindByKey(ctx, tx, key)
		if err != nil {
			return err
		}

		if existing != nil {
			result = &TransferResult{
				StatusCode: existing.StatusCode,
				Replayed:   true,
			}
			if existing.ResponseBody != "" {
				var body interface{}
				if jsonErr := json.Unmarshal([]byte(existing.ResponseBody), &body); jsonErr == nil {
					result.Body = body
				}
			}
			return nil
		}

		if err := s.executeTransferInTx(ctx, tx, req, amount); err != nil {
			return err
		}

		respBody, _ := json.Marshal(map[string]interface{}{})
		idempotencyEntry := &model.IdempotencyKey{
			Key:          key,
			StatusCode:   201,
			ResponseBody: string(respBody),
			ExpiresAt:    time.Now().Add(24 * time.Hour),
		}

		if err := s.idempotencyRepo.Create(ctx, tx, idempotencyEntry); err != nil {
			return err
		}

		result = &TransferResult{StatusCode: 201}
		return nil
	})

	if errors.Is(err, apperror.ErrIdempotencyKeyConflict) {
		// Another concurrent transaction already committed this key.
		// Our transfer was rolled back — plain SELECT re-fetch to get the committed value.
		existing, fetchErr := s.idempotencyRepo.GetByKey(ctx, key)
		if fetchErr != nil {
			return nil, fetchErr
		}
		if existing != nil {
			r := &TransferResult{StatusCode: existing.StatusCode, Replayed: true}
			if existing.ResponseBody != "" {
				var body interface{}
				if jsonErr := json.Unmarshal([]byte(existing.ResponseBody), &body); jsonErr == nil {
					r.Body = body
				}
			}
			return r, nil
		}
	}

	return result, err
}

func (s *transferService) executeTransfer(
	ctx context.Context,
	req dto.TransferRequest,
	amount decimal.Decimal,
) (*TransferResult, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return s.executeTransferInTx(ctx, tx, req, amount)
	})
	if err != nil {
		return nil, err
	}
	return &TransferResult{StatusCode: 201}, nil
}

// executeTransferInTx performs the actual fund transfer within a DB transaction.
// Locks are acquired in ascending account ID order to prevent deadlocks.
// Both accounts are fetched and updated in single batch queries.
func (s *transferService) executeTransferInTx(
	ctx context.Context,
	tx *gorm.DB,
	req dto.TransferRequest,
	amount decimal.Decimal,
) error {
	firstID, secondID := req.SourceAccountID, req.DestinationAccountID
	if firstID > secondID {
		firstID, secondID = secondID, firstID
	}

	// Single SELECT FOR UPDATE fetches both accounts in one round-trip.
	firstAccount, secondAccount, err := s.accountRepo.FindByIDsForUpdate(ctx, tx, firstID, secondID)
	if err != nil {
		return err
	}

	var source, dest *model.Account
	if firstID == req.SourceAccountID {
		source, dest = firstAccount, secondAccount
	} else {
		source, dest = secondAccount, firstAccount
	}

	if source.Balance.LessThan(amount) {
		return apperror.ErrInsufficientFunds
	}

	source.Balance = model.NewDecimal(source.Balance.Sub(amount))
	dest.Balance = model.NewDecimal(dest.Balance.Add(amount))

	// Single UPDATE sets both balances in one round-trip.
	if err := s.accountRepo.UpdateBalances(ctx, tx, source, dest); err != nil {
		return err
	}

	txn := &model.Transaction{
		SourceAccountID:      req.SourceAccountID,
		DestinationAccountID: req.DestinationAccountID,
		Amount:               model.NewDecimal(amount),
	}

	if err := s.transactionRepo.Create(ctx, tx, txn); err != nil {
		return err
	}

	s.logger.Debug("transfer executed",
		zap.Int64("source", req.SourceAccountID),
		zap.Int64("destination", req.DestinationAccountID),
		zap.String("amount", amount.String()),
	)

	return nil
}

func isRetryableError(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "40001": // serialization_failure
			return true
		case "40P01": // deadlock_detected
			return true
		}
	}
	return false
}
