package server

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap"
	radiuspkg "github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/radius"
	"layeh.com/radius"
)

// Handler はRADIUSリクエストを処理するハンドラ。
// layeh.com/radius.Handlerインターフェースの実装。
type Handler struct {
	engine eap.EAPProcessor
}

// NewHandler は新しいHandlerを生成する
func NewHandler(engine eap.EAPProcessor) *Handler {
	return &Handler{engine: engine}
}

// ServeRADIUS はRADIUSリクエストを処理する
func (h *Handler) ServeRADIUS(w radius.ResponseWriter, r *radius.Request) {
	traceID := uuid.New().String()
	srcIP := extractIP(r.RemoteAddr)

	slog.Info("RADIUSパケット受信",
		"event_id", "PKT_RECV",
		"trace_id", traceID,
		"src_ip", srcIP,
		"code", r.Code,
	)

	switch r.Code {
	case radius.CodeAccessRequest:
		h.handleAccessRequest(w, r, traceID, srcIP)

	case radius.CodeStatusServer:
		h.handleStatusServer(w, r, traceID, srcIP)

	default:
		slog.Warn("未対応のRADIUS Code",
			"event_id", "PKT_UNKNOWN_CODE",
			"trace_id", traceID,
			"code", r.Code,
		)
		// 応答なし
	}
}

// handleAccessRequest はAccess-Requestを処理する
func (h *Handler) handleAccessRequest(w radius.ResponseWriter, r *radius.Request, traceID, srcIP string) {
	secret := r.Packet.Secret

	// Message-Authenticator検証
	if !radiuspkg.VerifyMessageAuthenticator(r.Packet, secret) {
		slog.Warn("Message-Authenticator検証失敗",
			"event_id", "PKT_MA_INVALID",
			"trace_id", traceID,
			"src_ip", srcIP,
		)
		return // 応答なし
	}

	// EAP-Message抽出
	eapMessage, ok := radiuspkg.GetEAPMessage(r.Packet)
	if !ok {
		slog.Warn("EAP-Message属性なし",
			"event_id", "PKT_NO_EAP",
			"trace_id", traceID,
			"src_ip", srcIP,
		)
		return // 応答なし
	}

	// RADIUS属性抽出
	nasID, _ := radiuspkg.GetNASIdentifier(r.Packet)
	calledStation, _ := radiuspkg.GetCalledStationID(r.Packet)
	userName, _ := radiuspkg.GetUserName(r.Packet)
	state, _ := radiuspkg.GetState(r.Packet)

	// ProxyState抽出
	proxyStates := radiuspkg.ExtractProxyStates(r.Packet)

	// EAPリクエスト構築
	eapReq := &eap.Request{
		TraceID:       traceID,
		SrcIP:         srcIP,
		NASIdentifier: nasID,
		CalledStation: calledStation,
		UserName:      userName,
		State:         state,
		EAPMessage:    eapMessage,
	}

	// EAPエンジン処理
	ctx := context.Background()
	result, err := h.engine.Process(ctx, eapReq)
	if err != nil {
		slog.Error("EAPエンジンエラー",
			"event_id", "EAP_ENGINE_ERR",
			"trace_id", traceID,
			"error", err,
		)
		return // 応答なし
	}

	// 結果に基づいてRADIUS応答を構築
	switch result.Action {
	case eap.ActionAccept:
		resp := radiuspkg.BuildAccessAccept(r.Packet, secret, &radiuspkg.AcceptParams{
			EAPMessage:     result.EAPMessage,
			MSK:            result.MSK,
			SessionID:      result.SessionID,
			VlanID:         result.VlanID,
			SessionTimeout: result.SessionTimeout,
			ProxyStates:    proxyStates,
		})
		if err := w.Write(resp); err != nil {
			slog.Error("RADIUS応答送信失敗",
				"event_id", "PKT_SEND_ERR",
				"trace_id", traceID,
				"error", err,
			)
		}

	case eap.ActionChallenge:
		resp := radiuspkg.BuildAccessChallenge(r.Packet, secret, &radiuspkg.ChallengeParams{
			EAPMessage:  result.EAPMessage,
			State:       result.State,
			ProxyStates: proxyStates,
		})
		if err := w.Write(resp); err != nil {
			slog.Error("RADIUS応答送信失敗",
				"event_id", "PKT_SEND_ERR",
				"trace_id", traceID,
				"error", err,
			)
		}

	case eap.ActionReject:
		resp := radiuspkg.BuildAccessReject(r.Packet, secret, &radiuspkg.RejectParams{
			EAPMessage:  result.EAPMessage,
			ProxyStates: proxyStates,
		})
		if err := w.Write(resp); err != nil {
			slog.Error("RADIUS応答送信失敗",
				"event_id", "PKT_SEND_ERR",
				"trace_id", traceID,
				"error", err,
			)
		}

	case eap.ActionDrop:
		slog.Info("パケットドロップ",
			"event_id", "PKT_DROP",
			"trace_id", traceID,
		)
		// 応答なし
	}
}

// handleStatusServer はStatus-Serverリクエストに応答する。
// Message-Authenticator検証を行い、失敗時は無応答（破棄）とする。
func (h *Handler) handleStatusServer(w radius.ResponseWriter, r *radius.Request, traceID, srcIP string) {
	resp := radiuspkg.HandleStatusServer(r.Packet, r.Packet.Secret, srcIP, traceID)
	if resp == nil {
		return // Message-Authenticator検証失敗 → 無応答
	}

	if err := w.Write(resp); err != nil {
		slog.Error("Status-Server応答送信失敗",
			"event_id", "PKT_SEND_ERR",
			"trace_id", traceID,
			"error", err,
		)
	}
}
