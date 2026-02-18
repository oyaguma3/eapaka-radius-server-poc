package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
)

// HandleHealth はGET /health のハンドラー。
func (h *VectorHandler) HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, dto.HealthResponse{Status: "ok"})
}
