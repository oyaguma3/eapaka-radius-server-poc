package server

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/handler"
)

const traceIDHeader = "X-Trace-ID"

// TraceIDMiddleware はX-Trace-IDヘッダからトレースIDを取得する。
func TraceIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		traceID := c.GetHeader(traceIDHeader)
		if traceID == "" {
			traceID = "no-trace-id"
		}
		c.Set(handler.TraceIDKey, traceID)
		c.Next()
	}
}

// LoggingMiddleware はリクエストログを出力する。
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		latency := time.Since(start)
		traceID, _ := c.Get(handler.TraceIDKey)

		slog.Info("request completed",
			"trace_id", traceID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"http_status", c.Writer.Status(),
			"latency_ms", latency.Milliseconds(),
		)
	}
}

// RecoveryMiddleware はパニックからの復旧を行う。
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				traceID, _ := c.Get(handler.TraceIDKey)
				slog.Error("panic recovered",
					"trace_id", traceID,
					"error", err,
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, dto.NewProblemDetail(
					http.StatusInternalServerError,
					"Internal Server Error",
					"An unexpected error occurred",
				))
			}
		}()
		c.Next()
	}
}
