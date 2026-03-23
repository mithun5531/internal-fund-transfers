package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mithunp/internal-fund-transfers/internal/dto"
	"gorm.io/gorm"
)

type HealthHandler struct {
	db *gorm.DB
}

func NewHealthHandler(db *gorm.DB) *HealthHandler {
	return &HealthHandler{db: db}
}

func (h *HealthHandler) Check(c *gin.Context) {
	dbStatus := "connected"

	sqlDB, err := h.db.DB()
	if err != nil || sqlDB.Ping() != nil {
		dbStatus = "disconnected"
		c.JSON(http.StatusServiceUnavailable, dto.HealthResponse{
			Status: "unhealthy",
			DB:     dbStatus,
		})
		return
	}

	c.JSON(http.StatusOK, dto.HealthResponse{
		Status: "ok",
		DB:     dbStatus,
	})
}
