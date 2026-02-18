package server

import (
	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/handler"
)

// SetupRouter はルーティングを設定する。
func SetupRouter(engine *gin.Engine, h *handler.VectorHandler) {
	// ヘルスチェック
	engine.GET("/health", h.HandleHealth)

	// API v1
	v1 := engine.Group("/api/v1")
	{
		v1.POST("/vector", h.HandleVector)
	}
}
