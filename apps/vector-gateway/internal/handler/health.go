package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// healthResponse はヘルスチェックレスポンスを表す。
type healthResponse struct {
	Status string `json:"status"`
}

// HandleHealth はGET /health のハンドラー。
func (h *VectorHandler) HandleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, healthResponse{Status: "ok"})
}
