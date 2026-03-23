package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mithunp/internal-fund-transfers/internal/apperror"
	"github.com/mithunp/internal-fund-transfers/internal/dto"
	"github.com/mithunp/internal-fund-transfers/internal/service"
)

type TransactionHandler struct {
	transferService service.TransferService
}

func NewTransactionHandler(transferService service.TransferService) *TransactionHandler {
	return &TransactionHandler{transferService: transferService}
}

func (h *TransactionHandler) Create(c *gin.Context) {
	var req dto.TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: apperror.ErrInvalidRequestBody.Error()})
		return
	}

	idempotencyKey := c.GetHeader("Idempotency-Key")

	result, err := h.transferService.Transfer(c.Request.Context(), idempotencyKey, req)
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrAccountNotFound):
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, apperror.ErrInsufficientFunds):
			c.JSON(http.StatusUnprocessableEntity, dto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, apperror.ErrSameAccount):
			c.JSON(http.StatusUnprocessableEntity, dto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, apperror.ErrInvalidAmount):
			c.JSON(http.StatusUnprocessableEntity, dto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, apperror.ErrInvalidRequestBody):
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: apperror.ErrInternal.Error()})
		}
		return
	}

	statusCode := result.StatusCode
	if result.Replayed {
		statusCode = http.StatusOK
	}

	if result.Body != nil {
		c.JSON(statusCode, result.Body)
	} else {
		c.JSON(statusCode, gin.H{})
	}
}
