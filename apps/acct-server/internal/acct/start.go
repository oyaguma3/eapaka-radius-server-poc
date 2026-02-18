package acct

import (
	"context"
	"log/slog"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/session"
)

// ProcessStart はAcct-Start処理を行う。
func (p *Processor) ProcessStart(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
	// 1. 重複検出
	isDuplicate, err := p.duplicateDetector.CheckAndMarkStart(ctx, attrs.AcctSessionID)
	if err != nil {
		// SequenceError（Stop後Start）はログに出力して処理継続
		if seqErr, ok := err.(*SequenceError); ok {
			slog.Warn("sequence error",
				"event_id", "ACCT_SEQUENCE_ERR",
				"trace_id", traceID,
				"src_ip", srcIP,
				"acct_session_id", attrs.AcctSessionID,
				"reason", seqErr.Reason,
			)
		} else {
			slog.Error("duplicate check failed",
				"event_id", "VALKEY_CONN_ERR",
				"trace_id", traceID,
				"error", err.Error(),
			)
		}
	}
	if isDuplicate {
		slog.Warn("duplicate accounting start",
			"event_id", "ACCT_DUPLICATE_START",
			"trace_id", traceID,
			"src_ip", srcIP,
			"acct_session_id", attrs.AcctSessionID,
		)
		return nil
	}

	// 2. Class属性からセッションUUID取得
	sessionUUID := attrs.ClassUUID
	if sessionUUID == "" {
		slog.Warn("class attribute missing or invalid",
			"event_id", "ACCT_SESSION_NOT_FOUND",
			"trace_id", traceID,
			"src_ip", srcIP,
			"acct_session_id", attrs.AcctSessionID,
		)
	}

	// 3. セッション存在確認・更新
	if sessionUUID != "" {
		exists, err := p.sessionManager.Exists(ctx, sessionUUID)
		if err != nil {
			slog.Error("valkey error",
				"event_id", "VALKEY_CONN_ERR",
				"trace_id", traceID,
				"error", err.Error(),
			)
		} else if !exists {
			slog.Warn("session not found",
				"event_id", "ACCT_SESSION_NOT_FOUND",
				"trace_id", traceID,
				"src_ip", srcIP,
				"class_uuid", sessionUUID,
			)
		} else {
			err = p.sessionManager.UpdateOnStart(ctx, sessionUUID, &session.SessionStartData{
				StartTime: time.Now().Unix(),
				NasIP:     srcIP,
				AcctID:    attrs.AcctSessionID,
				ClientIP:  attrs.FramedIPAddress,
			})
			if err != nil {
				slog.Error("session update failed",
					"event_id", "DB_WRITE_ERR",
					"trace_id", traceID,
					"error", err.Error(),
				)
			}
		}
	}

	// 4. ログ出力
	imsi := p.identifierResolver.ResolveIMSI(ctx, sessionUUID, attrs.UserName, attrs.ClassUUID)
	slog.Info("accounting start",
		"event_id", "ACCT_START",
		"trace_id", traceID,
		"src_ip", srcIP,
		"imsi", imsi,
		"acct_session_id", attrs.AcctSessionID,
	)

	return nil
}
