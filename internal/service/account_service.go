package service

import (
	"context"

	"github.com/mithunp/internal-fund-transfers/internal/apperror"
	"github.com/mithunp/internal-fund-transfers/internal/dto"
	"github.com/mithunp/internal-fund-transfers/internal/model"
	"github.com/mithunp/internal-fund-transfers/internal/repository"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type AccountService interface {
	Create(ctx context.Context, req dto.CreateAccountRequest) error
	GetByID(ctx context.Context, id int64) (*dto.AccountResponse, error)
}

type accountService struct {
	accountRepo repository.AccountRepository
	logger      *zap.Logger
}

func NewAccountService(accountRepo repository.AccountRepository, logger *zap.Logger) AccountService {
	return &accountService{
		accountRepo: accountRepo,
		logger:      logger,
	}
}

func (s *accountService) Create(ctx context.Context, req dto.CreateAccountRequest) error {
	balance, err := decimal.NewFromString(req.InitialBalance)
	if err != nil {
		return apperror.ErrInvalidRequestBody
	}

	if balance.IsNegative() {
		return apperror.ErrNegativeBalance
	}

	account := &model.Account{
		ID:      req.AccountID,
		Balance: model.NewDecimal(balance),
	}

	if err := s.accountRepo.Create(ctx, account); err != nil {
		return err
	}

	s.logger.Info("account created",
		zap.Int64("account_id", req.AccountID),
		zap.String("initial_balance", balance.String()),
	)

	return nil
}

func (s *accountService) GetByID(ctx context.Context, id int64) (*dto.AccountResponse, error) {
	account, err := s.accountRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &dto.AccountResponse{
		AccountID: account.ID,
		Balance:   account.Balance.String(),
	}, nil
}
