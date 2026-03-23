package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mithunp/internal-fund-transfers/internal/apperror"
	"github.com/mithunp/internal-fund-transfers/internal/dto"
	"github.com/mithunp/internal-fund-transfers/internal/service"
)

type AccountHandler struct {
	accountService service.AccountService
}

func NewAccountHandler(accountService service.AccountService) *AccountHandler {
	return &AccountHandler{accountService: accountService}
}

func (h *AccountHandler) Create(c *gin.Context) {
	var req dto.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: apperror.ErrInvalidRequestBody.Error()})
		return
	}

	if err := h.accountService.Create(c.Request.Context(), req); err != nil {
		switch {
		case errors.Is(err, apperror.ErrAccountExists):
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, apperror.ErrNegativeBalance):
			c.JSON(http.StatusUnprocessableEntity, dto.ErrorResponse{Error: err.Error()})
		case errors.Is(err, apperror.ErrInvalidRequestBody):
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: apperror.ErrInternal.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{})
}

func (h *AccountHandler) GetByID(c *gin.Context) {
	idStr := c.Param("account_id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: apperror.ErrInvalidAccountID.Error()})
		return
	}

	resp, err := h.accountService.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, apperror.ErrAccountNotFound) {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: apperror.ErrInternal.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
