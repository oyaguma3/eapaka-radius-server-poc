package acct

import (
	"context"
	"log/slog"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/session"
)

// ProcessInterim はAcct-Interim処理を行う。
func (p *Processor) ProcessInterim(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) {
	// 1. 重複・順序異常の判定
	check, err := p.duplicateDetector.CheckInterim(ctx, attrs.AcctSessionID, attrs.InputOctets, attrs.OutputOctets)
	if err != nil {
		slog.Error("duplicate check failed",
			"event_id", "VALKEY_CONN_ERR",
			"trace_id", traceID,
			"error", err.Error(),
		)
	}
	if check.Duplicate {
		slog.Warn("duplicate accounting interim",
			"event_id", "ACCT_DUPLICATE_INTERIM",
			"trace_id", traceID,
			"src_ip", srcIP,
			"nas_identifier", attrs.NasIdentifier,
			"acct_session_id", attrs.AcctSessionID,
		)
		return
	}
	if check.SequenceReason != "" {
		// 順序異常でも課金データの欠損を避けるため処理を継続する
		slog.Warn("interim sequence error",
			"event_id", "ACCT_SEQUENCE_ERR",
			"trace_id", traceID,
			"src_ip", srcIP,
			"nas_identifier", attrs.NasIdentifier,
			"acct_session_id", attrs.AcctSessionID,
			"reason", check.SequenceReason,
		)
	}

	// 2. セッション更新（存在するセッションのみ。不在のキーを作らない）
	sessionUUID := attrs.ClassUUID
	if sessionUUID != "" {
		exists, err := p.sessionManager.Exists(ctx, sessionUUID)
		switch {
		case err != nil:
			slog.Error("valkey error",
				"event_id", "VALKEY_CONN_ERR",
				"trace_id", traceID,
				"error", err.Error(),
			)
		case !exists:
			slog.Warn("session not found",
				"event_id", "ACCT_SESSION_NOT_FOUND",
				"trace_id", traceID,
				"src_ip", srcIP,
				"nas_identifier", attrs.NasIdentifier,
				"class_uuid", sessionUUID,
			)
		default:
			err = p.sessionManager.UpdateOnInterim(ctx, sessionUUID, &session.SessionInterimData{
				NasIP:         srcIP,
				NasIdentifier: attrs.NasIdentifier,
				ClientIP:      attrs.FramedIPAddress,
				InputOctets:   int64(attrs.InputOctets),
				OutputOctets:  int64(attrs.OutputOctets),
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

	// 3. ログ出力
	imsi := p.identifierResolver.ResolveIMSI(ctx, sessionUUID, attrs.UserName, attrs.ClassUUID)
	slog.Info("accounting interim",
		"event_id", "ACCT_INTERIM",
		"trace_id", traceID,
		"src_ip", srcIP,
		"nas_identifier", attrs.NasIdentifier,
		"imsi", imsi,
		"acct_session_id", attrs.AcctSessionID,
		"input_octets", attrs.InputOctets,
		"output_octets", attrs.OutputOctets,
	)
}
