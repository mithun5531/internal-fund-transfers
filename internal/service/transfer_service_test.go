package service_test

import (
	"context"
	"testing"

	"github.com/mithunp/internal-fund-transfers/internal/apperror"
	"github.com/mithunp/internal-fund-transfers/internal/dto"
	"github.com/mithunp/internal-fund-transfers/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestTransferValidation_SameAccount(t *testing.T) {
	svc := service.NewTransferService(nil, nil, nil, nil, 0, nil)
	_, err := svc.Transfer(context.Background(), "", dto.TransferRequest{
		SourceAccountID:      1,
		DestinationAccountID: 1,
		Amount:               "100.00",
	})
	assert.ErrorIs(t, err, apperror.ErrSameAccount)
}

func TestTransferValidation_NegativeAmount(t *testing.T) {
	svc := service.NewTransferService(nil, nil, nil, nil, 0, nil)
	_, err := svc.Transfer(context.Background(), "", dto.TransferRequest{
		SourceAccountID:      1,
		DestinationAccountID: 2,
		Amount:               "-50.00",
	})
	assert.ErrorIs(t, err, apperror.ErrInvalidAmount)
}

func TestTransferValidation_ZeroAmount(t *testing.T) {
	svc := service.NewTransferService(nil, nil, nil, nil, 0, nil)
	_, err := svc.Transfer(context.Background(), "", dto.TransferRequest{
		SourceAccountID:      1,
		DestinationAccountID: 2,
		Amount:               "0",
	})
	assert.ErrorIs(t, err, apperror.ErrInvalidAmount)
}

func TestTransferValidation_InvalidAmount(t *testing.T) {
	svc := service.NewTransferService(nil, nil, nil, nil, 0, nil)
	_, err := svc.Transfer(context.Background(), "", dto.TransferRequest{
		SourceAccountID:      1,
		DestinationAccountID: 2,
		Amount:               "not-a-number",
	})
	assert.ErrorIs(t, err, apperror.ErrInvalidRequestBody)
}
