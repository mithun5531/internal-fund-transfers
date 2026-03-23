package apperror

import "errors"

var (
	ErrAccountNotFound    = errors.New("account not found")
	ErrAccountExists      = errors.New("account already exists")
	ErrInsufficientFunds  = errors.New("insufficient funds")
	ErrSameAccount        = errors.New("cannot transfer to same account")
	ErrInvalidAmount      = errors.New("amount must be positive")
	ErrNegativeBalance    = errors.New("initial balance must not be negative")
	ErrInvalidRequestBody = errors.New("invalid request body")
	ErrInvalidAccountID   = errors.New("invalid account ID")
	ErrInternal               = errors.New("internal server error")
	ErrIdempotencyKeyConflict = errors.New("idempotency key already committed")
)
