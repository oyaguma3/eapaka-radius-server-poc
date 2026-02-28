// Package handler はHTTPリクエストハンドラーを提供する。
package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/backend"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/router"
)

// TraceIDKey はコンテキストにTraceIDを格納するキー。
const TraceIDKey = "trace_id"

// VectorHandler はベクター転送APIのハンドラー。
type VectorHandler struct {
	router *router.Router
	cfg    *config.Config
}

// NewVectorHandler は新しいVectorHandlerを生成する。
func NewVectorHandler(r *router.Router, cfg *config.Config) *VectorHandler {
	return &VectorHandler{
		router: r,
		cfg:    cfg,
	}
}

// HandleVector はPOST /api/v1/vector のハンドラー。
func (h *VectorHandler) HandleVector(c *gin.Context) {
	traceID, _ := c.Get(TraceIDKey)
	ctx := backend.ContextWithTraceID(c.Request.Context(), fmt.Sprint(traceID))

	// 1. リクエストバインド
	var req backend.VectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request body",
			"trace_id", traceID,
			"event_id", "GW_ERR",
			"error", err.Error(),
		)
		c.JSON(http.StatusBadRequest, backend.NewProblemDetail(
			http.StatusBadRequest,
			"Bad Request",
			"Invalid request body",
		))
		return
	}

	// 2. IMSI検証
	if err := validateIMSI(req.IMSI); err != nil {
		slog.Warn("invalid IMSI format",
			"trace_id", traceID,
			"event_id", "GW_ERR",
			"imsi", logging.MaskIMSI(req.IMSI, h.cfg.LogMaskIMSI),
			"error", err.Error(),
		)
		c.JSON(http.StatusBadRequest, backend.NewProblemDetail(
			http.StatusBadRequest,
			"Bad Request",
			"IMSI must be 15 digits",
		))
		return
	}

	// 3. バックエンド選択
	b, err := h.router.SelectBackend(req.IMSI)
	if err != nil {
		h.handleRoutingError(c, traceID, req.IMSI, err)
		return
	}

	slog.Info("backend selected",
		"trace_id", traceID,
		"event_id", "GW_ROUTE",
		"imsi", logging.MaskIMSI(req.IMSI, h.cfg.LogMaskIMSI),
		"backend_id", b.ID(),
		"backend_name", b.Name(),
	)

	// 4. ベクター取得
	resp, err := b.GetVector(ctx, &req)
	if err != nil {
		h.handleBackendError(c, traceID, req.IMSI, err)
		return
	}

	// 5. 成功レスポンス
	slog.Info("vector forwarded",
		"trace_id", traceID,
		"event_id", "GW_OK",
		"imsi", logging.MaskIMSI(req.IMSI, h.cfg.LogMaskIMSI),
		"backend_id", b.ID(),
		"http_status", http.StatusOK,
	)
	c.JSON(http.StatusOK, resp)
}

// handleRoutingError はルーティングエラーを処理する。
func (h *VectorHandler) handleRoutingError(c *gin.Context, traceID any, imsi string, err error) {
	var notImpl *backend.BackendNotImplementedError
	if errors.As(err, &notImpl) {
		slog.Warn("backend not implemented",
			"trace_id", traceID,
			"event_id", "GW_ERR",
			"imsi", logging.MaskIMSI(imsi, h.cfg.LogMaskIMSI),
			"backend_id", notImpl.ID,
		)
		c.JSON(http.StatusNotImplemented, backend.NewProblemDetail(
			http.StatusNotImplemented,
			"Not Implemented",
			fmt.Sprintf("Backend %q is not implemented", notImpl.ID),
		))
		return
	}

	slog.Error("routing error",
		"trace_id", traceID,
		"event_id", "GW_ERR",
		"imsi", logging.MaskIMSI(imsi, h.cfg.LogMaskIMSI),
		"error", err.Error(),
	)
	c.JSON(http.StatusInternalServerError, backend.NewProblemDetail(
		http.StatusInternalServerError,
		"Internal Server Error",
		"An unexpected error occurred",
	))
}

// handleBackendError はバックエンドエラーを処理する。
func (h *VectorHandler) handleBackendError(c *gin.Context, traceID any, imsi string, err error) {
	// 4xxエラー: そのまま伝搬
	var respErr *backend.BackendResponseError
	if errors.As(err, &respErr) {
		slog.Warn("backend returned error",
			"trace_id", traceID,
			"event_id", "GW_ERR",
			"imsi", logging.MaskIMSI(imsi, h.cfg.LogMaskIMSI),
			"http_status", respErr.StatusCode,
		)
		c.JSON(respErr.StatusCode, respErr.Problem)
		return
	}

	// 通信エラー: 502
	var commErr *backend.BackendCommunicationError
	if errors.As(err, &commErr) {
		slog.Error("backend communication error",
			"trace_id", traceID,
			"event_id", "GW_ERR",
			"imsi", logging.MaskIMSI(imsi, h.cfg.LogMaskIMSI),
			"error", commErr.Error(),
		)
		c.JSON(http.StatusBadGateway, backend.NewProblemDetail(
			http.StatusBadGateway,
			"Bad Gateway",
			"Failed to communicate with backend service",
		))
		return
	}

	// その他: 500
	slog.Error("unexpected backend error",
		"trace_id", traceID,
		"event_id", "GW_ERR",
		"imsi", logging.MaskIMSI(imsi, h.cfg.LogMaskIMSI),
		"error", err.Error(),
	)
	c.JSON(http.StatusInternalServerError, backend.NewProblemDetail(
		http.StatusInternalServerError,
		"Internal Server Error",
		"An unexpected error occurred",
	))
}

// validateIMSI はIMSI形式を検証する。
func validateIMSI(imsi string) error {
	if len(imsi) != 15 {
		return fmt.Errorf("IMSI must be 15 digits, got %d", len(imsi))
	}
	for _, c := range imsi {
		if c < '0' || c > '9' {
			return fmt.Errorf("IMSI must contain only digits")
		}
	}
	return nil
}
