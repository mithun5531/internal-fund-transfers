package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mithunp/internal-fund-transfers/internal/apperror"
	"github.com/mithunp/internal-fund-transfers/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IdempotencyRepository interface {
	FindByKey(ctx context.Context, tx *gorm.DB, key string) (*model.IdempotencyKey, error)
	GetByKey(ctx context.Context, key string) (*model.IdempotencyKey, error)
	Create(ctx context.Context, tx *gorm.DB, entry *model.IdempotencyKey) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type idempotencyRepository struct {
	db *gorm.DB
}

func NewIdempotencyRepository(db *gorm.DB) IdempotencyRepository {
	return &idempotencyRepository{db: db}
}

// FindByKey looks up an idempotency key with FOR UPDATE SKIP LOCKED.
// If another transaction holds the lock (e.g. mid-commit), the row is
// skipped and nil is returned — the caller proceeds as if the key is new,
// executes the transfer, and fails on the idempotency key INSERT (23505),
// which the ErrIdempotencyKeyConflict handler resolves via GetByKey.
func (r *idempotencyRepository) FindByKey(ctx context.Context, tx *gorm.DB, key string) (*model.IdempotencyKey, error) {
	var entry model.IdempotencyKey
	result := tx.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
		Where("key = ?", key).
		First(&entry)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &entry, nil
}

// GetByKey performs a plain SELECT (no lock) to read a committed idempotency key.
// Use this for conflict-replay re-fetches where locking is not required.
func (r *idempotencyRepository) GetByKey(ctx context.Context, key string) (*model.IdempotencyKey, error) {
	var entry model.IdempotencyKey
	result := r.db.WithContext(ctx).Where("key = ?", key).First(&entry)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &entry, nil
}

func (r *idempotencyRepository) Create(ctx context.Context, tx *gorm.DB, entry *model.IdempotencyKey) error {
	result := tx.WithContext(ctx).Create(entry)
	if result.Error != nil {
		var pgErr *pgconn.PgError
		if errors.As(result.Error, &pgErr) && pgErr.Code == "23505" {
			return apperror.ErrIdempotencyKeyConflict
		}
		return result.Error
	}
	return nil
}

func (r *idempotencyRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("expires_at < ?", time.Now()).
		Delete(&model.IdempotencyKey{})
	return result.RowsAffected, result.Error
}
