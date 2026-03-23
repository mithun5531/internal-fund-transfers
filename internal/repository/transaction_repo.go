package repository

import (
	"context"

	"github.com/mithunp/internal-fund-transfers/internal/model"
	"gorm.io/gorm"
)

type TransactionRepository interface {
	Create(ctx context.Context, tx *gorm.DB, transaction *model.Transaction) error
}

type transactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) TransactionRepository {
	return &transactionRepository{db: db}
}

func (r *transactionRepository) Create(ctx context.Context, tx *gorm.DB, transaction *model.Transaction) error {
	result := tx.WithContext(ctx).Create(transaction)
	return result.Error
}
