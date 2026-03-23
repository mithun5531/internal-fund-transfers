package service_test

import (
	"context"
	"testing"

	"github.com/mithunp/internal-fund-transfers/internal/apperror"
	"github.com/mithunp/internal-fund-transfers/internal/dto"
	"github.com/mithunp/internal-fund-transfers/internal/model"
	"github.com/mithunp/internal-fund-transfers/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type stubAccountRepo struct {
	createFunc          func(account *model.Account) error
	findByIDFunc        func(id int64) (*model.Account, error)
	findByIDForUpdateFn func(tx *gorm.DB, id int64) (*model.Account, error)
	updateBalanceFn     func(tx *gorm.DB, account *model.Account) error
}

func (s *stubAccountRepo) Create(_ context.Context, account *model.Account) error {
	if s.createFunc != nil {
		return s.createFunc(account)
	}
	return nil
}

func (s *stubAccountRepo) FindByID(_ context.Context, id int64) (*model.Account, error) {
	if s.findByIDFunc != nil {
		return s.findByIDFunc(id)
	}
	return nil, apperror.ErrAccountNotFound
}

func (s *stubAccountRepo) FindByIDForUpdate(_ context.Context, tx *gorm.DB, id int64) (*model.Account, error) {
	if s.findByIDForUpdateFn != nil {
		return s.findByIDForUpdateFn(tx, id)
	}
	return nil, apperror.ErrAccountNotFound
}

func (s *stubAccountRepo) FindByIDsForUpdate(_ context.Context, _ *gorm.DB, _, _ int64) (*model.Account, *model.Account, error) {
	return nil, nil, apperror.ErrAccountNotFound
}

func (s *stubAccountRepo) UpdateBalance(_ context.Context, tx *gorm.DB, account *model.Account) error {
	if s.updateBalanceFn != nil {
		return s.updateBalanceFn(tx, account)
	}
	return nil
}

func (s *stubAccountRepo) UpdateBalances(_ context.Context, _ *gorm.DB, _, _ *model.Account) error {
	return nil
}

func TestAccountService_Create_Success(t *testing.T) {
	repo := &stubAccountRepo{
		createFunc: func(account *model.Account) error {
			assert.Equal(t, int64(1), account.ID)
			assert.True(t, account.Balance.Equal(decimal.RequireFromString("100.50")))
			return nil
		},
	}

	svc := service.NewAccountService(repo, zap.NewNop())
	err := svc.Create(context.Background(), dto.CreateAccountRequest{
		AccountID:      1,
		InitialBalance: "100.50",
	})

	require.NoError(t, err)
}

func TestAccountService_Create_NegativeBalance(t *testing.T) {
	repo := &stubAccountRepo{}
	svc := service.NewAccountService(repo, zap.NewNop())

	err := svc.Create(context.Background(), dto.CreateAccountRequest{
		AccountID:      1,
		InitialBalance: "-50.00",
	})

	assert.ErrorIs(t, err, apperror.ErrNegativeBalance)
}

func TestAccountService_Create_InvalidBalance(t *testing.T) {
	repo := &stubAccountRepo{}
	svc := service.NewAccountService(repo, zap.NewNop())

	err := svc.Create(context.Background(), dto.CreateAccountRequest{
		AccountID:      1,
		InitialBalance: "not-a-number",
	})

	assert.ErrorIs(t, err, apperror.ErrInvalidRequestBody)
}

func TestAccountService_Create_Duplicate(t *testing.T) {
	repo := &stubAccountRepo{
		createFunc: func(_ *model.Account) error {
			return apperror.ErrAccountExists
		},
	}

	svc := service.NewAccountService(repo, zap.NewNop())
	err := svc.Create(context.Background(), dto.CreateAccountRequest{
		AccountID:      1,
		InitialBalance: "100.00",
	})

	assert.ErrorIs(t, err, apperror.ErrAccountExists)
}

func TestAccountService_Create_ZeroBalance(t *testing.T) {
	repo := &stubAccountRepo{
		createFunc: func(account *model.Account) error {
			assert.True(t, account.Balance.Equal(decimal.Zero))
			return nil
		},
	}

	svc := service.NewAccountService(repo, zap.NewNop())
	err := svc.Create(context.Background(), dto.CreateAccountRequest{
		AccountID:      1,
		InitialBalance: "0",
	})

	require.NoError(t, err)
}

func TestAccountService_GetByID_Success(t *testing.T) {
	repo := &stubAccountRepo{
		findByIDFunc: func(id int64) (*model.Account, error) {
			return &model.Account{
				ID:      id,
				Balance: model.NewDecimal(decimal.RequireFromString("250.12345")),
			}, nil
		},
	}

	svc := service.NewAccountService(repo, zap.NewNop())
	resp, err := svc.GetByID(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, int64(1), resp.AccountID)
	assert.Equal(t, "250.12345", resp.Balance)
}

func TestAccountService_GetByID_NotFound(t *testing.T) {
	repo := &stubAccountRepo{
		findByIDFunc: func(_ int64) (*model.Account, error) {
			return nil, apperror.ErrAccountNotFound
		},
	}

	svc := service.NewAccountService(repo, zap.NewNop())
	_, err := svc.GetByID(context.Background(), 999)

	assert.ErrorIs(t, err, apperror.ErrAccountNotFound)
}

func TestAccountService_Create_PrecisionPreserved(t *testing.T) {
	repo := &stubAccountRepo{
		createFunc: func(account *model.Account) error {
			assert.Equal(t, "0.00001", account.Balance.String())
			return nil
		},
	}

	svc := service.NewAccountService(repo, zap.NewNop())
	err := svc.Create(context.Background(), dto.CreateAccountRequest{
		AccountID:      1,
		InitialBalance: "0.00001",
	})

	require.NoError(t, err)
}
