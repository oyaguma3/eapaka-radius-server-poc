// Package handler はHTTPリクエストハンドラーを提供する。
package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/usecase"
)

// TraceIDKey はコンテキストにTraceIDを格納するキー。
const TraceIDKey = "trace_id"

// VectorHandler はベクター生成APIのハンドラー。
type VectorHandler struct {
	useCase usecase.VectorUseCaseInterface
	cfg     *config.Config
}

// NewVectorHandler は新しいVectorHandlerを生成する。
func NewVectorHandler(useCase usecase.VectorUseCaseInterface, cfg *config.Config) *VectorHandler {
	return &VectorHandler{
		useCase: useCase,
		cfg:     cfg,
	}
}

// HandleVector はPOST /api/v1/vector のハンドラー。
func (h *VectorHandler) HandleVector(c *gin.Context) {
	traceID, _ := c.Get(TraceIDKey)
	ctx := c.Request.Context()

	// 1. リクエストバインド
	var req dto.VectorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Warn("invalid request body",
			"trace_id", traceID,
			"event_id", "CALC_ERR",
			"error", err.Error(),
		)
		c.JSON(http.StatusBadRequest, dto.NewProblemDetail(
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
			"event_id", "CALC_ERR",
			"imsi", h.maskIMSI(req.IMSI),
			"error", err.Error(),
		)
		c.JSON(http.StatusBadRequest, dto.NewProblemDetail(
			http.StatusBadRequest,
			"Bad Request",
			"IMSI must be 15 digits",
		))
		return
	}

	// 3. ユースケース実行
	resp, err := h.useCase.GenerateVector(ctx, &req)
	if err != nil {
		h.handleError(c, traceID, req.IMSI, err)
		return
	}

	// 4. 成功レスポンス
	slog.Info("vector generated",
		"trace_id", traceID,
		"event_id", "CALC_OK",
		"imsi", h.maskIMSI(req.IMSI),
		"http_status", http.StatusOK,
	)
	c.JSON(http.StatusOK, resp)
}

// handleError はエラーレスポンスを処理する。
func (h *VectorHandler) handleError(c *gin.Context, traceID any, imsi string, err error) {
	var problemErr *usecase.ProblemError
	if errors.As(err, &problemErr) {
		slog.Log(c.Request.Context(), problemErr.LogLevel(), problemErr.Message,
			"trace_id", traceID,
			"event_id", problemErr.EventID,
			"imsi", h.maskIMSI(imsi),
			"http_status", problemErr.Status,
		)
		c.JSON(problemErr.Status, problemErr.ToProblemDetail())
		return
	}

	// 予期しないエラー
	slog.Error("unexpected error",
		"trace_id", traceID,
		"event_id", "CALC_ERR",
		"imsi", h.maskIMSI(imsi),
		"error", err.Error(),
	)
	c.JSON(http.StatusInternalServerError, dto.NewProblemDetail(
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

// maskIMSI はログ出力用にIMSIをマスクする。
func (h *VectorHandler) maskIMSI(imsi string) string {
	if !h.cfg.LogMaskIMSI {
		return imsi
	}
	if len(imsi) <= 7 {
		return imsi
	}
	return imsi[:5] + "********" + imsi[len(imsi)-2:]
}
