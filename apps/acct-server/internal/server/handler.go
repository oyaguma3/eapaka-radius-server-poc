package server

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/acct"
	radiuspkg "github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
	"layeh.com/radius"
)

// Handler はRADIUSリクエストを処理するハンドラ。
type Handler struct {
	processor acct.AccountingProcessor
}

// NewHandler は新しいHandlerを生成する
func NewHandler(processor acct.AccountingProcessor) *Handler {
	return &Handler{processor: processor}
}

// ServeRADIUS はRADIUSリクエストを処理する
func (h *Handler) ServeRADIUS(w radius.ResponseWriter, r *radius.Request) {
	traceID := uuid.New().String()
	srcIP := extractIP(r.RemoteAddr)

	switch r.Code {
	case radius.CodeAccountingRequest:
		h.handleAccountingRequest(w, r, traceID, srcIP)

	case radius.CodeStatusServer:
		h.handleStatusServer(w, r, traceID, srcIP)

	default:
		slog.Warn("未対応のRADIUS Code",
			"event_id", "RADIUS_UNKNOWN_CODE",
			"trace_id", traceID,
			"src_ip", srcIP,
			"code", r.Code,
		)
	}
}

// handleAccountingRequest はAccounting-Requestを処理する
func (h *Handler) handleAccountingRequest(w radius.ResponseWriter, r *radius.Request, traceID, srcIP string) {
	secret := r.Secret

	// 1. Request Authenticator検証
	if !radiuspkg.VerifyAccountingAuthenticator(r.Packet, secret) {
		slog.Warn("Authenticator検証失敗",
			"event_id", "RADIUS_AUTH_ERR",
			"trace_id", traceID,
			"src_ip", srcIP,
		)
		return // パケット破棄
	}

	// 2. 属性抽出
	attrs, err := radiuspkg.ExtractAccountingAttributes(r.Packet)
	if err != nil {
		slog.Warn("属性抽出失敗",
			"event_id", "RADIUS_PARSE_ERR",
			"trace_id", traceID,
			"src_ip", srcIP,
			"reason", err.Error(),
		)
		return // パケット破棄
	}

	// 3. Status-Type別処理
	ctx := context.Background()
	var procErr error
	switch attrs.AcctStatusType {
	case radiuspkg.AcctStatusTypeStart:
		procErr = h.processor.ProcessStart(ctx, attrs, srcIP, traceID)
	case radiuspkg.AcctStatusTypeStop:
		procErr = h.processor.ProcessStop(ctx, attrs, srcIP, traceID)
	case radiuspkg.AcctStatusTypeInterim:
		procErr = h.processor.ProcessInterim(ctx, attrs, srcIP, traceID)
	default:
		slog.Warn("未対応のAcct-Status-Type",
			"event_id", "RADIUS_UNKNOWN_CODE",
			"trace_id", traceID,
			"src_ip", srcIP,
			"acct_status_type", attrs.AcctStatusType,
		)
		return // パケット破棄
	}

	// 4. 処理エラーがあってもAccounting-Responseは返す
	if procErr != nil {
		slog.Error("処理エラー",
			"event_id", "SYS_ERR",
			"trace_id", traceID,
			"error", procErr.Error(),
		)
	}

	// 5. Accounting-Response生成・送信
	response := radiuspkg.BuildAccountingResponse(r.Packet, attrs.ProxyStates)
	if err := w.Write(response); err != nil {
		slog.Error("RADIUS応答送信失敗",
			"event_id", "PKT_SEND_ERR",
			"trace_id", traceID,
			"error", err,
		)
	}
}

// handleStatusServer はStatus-Serverリクエストに応答する
func (h *Handler) handleStatusServer(w radius.ResponseWriter, r *radius.Request, traceID, srcIP string) {
	resp := radiuspkg.HandleStatusServer(r.Packet, r.Secret, srcIP, traceID)
	if resp == nil {
		return
	}
	if err := w.Write(resp); err != nil {
		slog.Error("Status-Server応答送信失敗",
			"event_id", "PKT_SEND_ERR",
			"trace_id", traceID,
			"error", err,
		)
	}
}
