package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mithunp/internal-fund-transfers/internal/apperror"
	"github.com/mithunp/internal-fund-transfers/internal/model"
	"gorm.io/gorm"
)

type AccountRepository interface {
	Create(ctx context.Context, account *model.Account) error
	FindByID(ctx context.Context, id int64) (*model.Account, error)
	FindByIDForUpdate(ctx context.Context, tx *gorm.DB, id int64) (*model.Account, error)
	FindByIDsForUpdate(ctx context.Context, tx *gorm.DB, id1, id2 int64) (*model.Account, *model.Account, error)
	UpdateBalance(ctx context.Context, tx *gorm.DB, account *model.Account) error
	UpdateBalances(ctx context.Context, tx *gorm.DB, a, b *model.Account) error
}

type accountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) AccountRepository {
	return &accountRepository{db: db}
}

func (r *accountRepository) Create(ctx context.Context, account *model.Account) error {
	result := r.db.WithContext(ctx).Create(account)
	if result.Error != nil {
		var pgErr *pgconn.PgError
		if errors.As(result.Error, &pgErr) && pgErr.Code == "23505" {
			return apperror.ErrAccountExists
		}
		return result.Error
	}
	return nil
}

func (r *accountRepository) FindByID(ctx context.Context, id int64) (*model.Account, error) {
	var account model.Account
	result := r.db.WithContext(ctx).First(&account, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrAccountNotFound
		}
		return nil, result.Error
	}
	return &account, nil
}

// FindByIDForUpdate acquires a row-level exclusive lock on the account row.
// Must be called within an active DB transaction.
func (r *accountRepository) FindByIDForUpdate(ctx context.Context, tx *gorm.DB, id int64) (*model.Account, error) {
	var account model.Account
	result := tx.WithContext(ctx).
		Clauses(forUpdate()).
		First(&account, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, apperror.ErrAccountNotFound
		}
		return nil, result.Error
	}
	return &account, nil
}

// FindByIDsForUpdate fetches two accounts with a single SELECT FOR UPDATE, ordered
// by id ascending to match the deadlock-prevention lock ordering. id1 must be < id2.
func (r *accountRepository) FindByIDsForUpdate(ctx context.Context, tx *gorm.DB, id1, id2 int64) (*model.Account, *model.Account, error) {
	var accounts []model.Account
	result := tx.WithContext(ctx).
		Clauses(forUpdate()).
		Where("id IN ?", []int64{id1, id2}).
		Order("id ASC").
		Find(&accounts)
	if result.Error != nil {
		return nil, nil, result.Error
	}
	if len(accounts) < 2 {
		return nil, nil, apperror.ErrAccountNotFound
	}
	return &accounts[0], &accounts[1], nil
}

func (r *accountRepository) UpdateBalance(ctx context.Context, tx *gorm.DB, account *model.Account) error {
	result := tx.WithContext(ctx).
		Model(account).
		Updates(map[string]interface{}{
			"balance": account.Balance,
			"version": gorm.Expr("version + 1"),
		})
	return result.Error
}

// UpdateBalances updates both account balances in a single UPDATE statement.
func (r *accountRepository) UpdateBalances(ctx context.Context, tx *gorm.DB, a, b *model.Account) error {
	return tx.WithContext(ctx).Exec(
		`UPDATE accounts SET balance = CASE id WHEN ? THEN ?::numeric WHEN ? THEN ?::numeric END, version = version + 1 WHERE id IN (?, ?)`,
		a.ID, a.Balance, b.ID, b.Balance, a.ID, b.ID,
	).Error
}
