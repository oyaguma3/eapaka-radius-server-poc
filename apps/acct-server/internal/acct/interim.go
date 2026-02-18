package acct

import (
	"context"
	"log/slog"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/session"
)

// ProcessInterim はAcct-Interim処理を行う。
func (p *Processor) ProcessInterim(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
	// 1. 重複検出
	isDuplicate, err := p.duplicateDetector.CheckInterimDuplicate(ctx, attrs.AcctSessionID, attrs.InputOctets, attrs.OutputOctets)
	if err != nil {
		slog.Error("duplicate check failed",
			"event_id", "VALKEY_CONN_ERR",
			"trace_id", traceID,
			"error", err.Error(),
		)
	}
	if isDuplicate {
		slog.Warn("duplicate accounting interim",
			"event_id", "ACCT_DUPLICATE_START",
			"trace_id", traceID,
			"src_ip", srcIP,
			"acct_session_id", attrs.AcctSessionID,
		)
		return nil
	}

	// 2. Startなしチェック
	seenStart, err := p.duplicateDetector.HasSeenStart(ctx, attrs.AcctSessionID)
	if err != nil {
		slog.Error("valkey error",
			"event_id", "VALKEY_CONN_ERR",
			"trace_id", traceID,
			"error", err.Error(),
		)
	} else if !seenStart {
		slog.Warn("interim without start",
			"event_id", "ACCT_SEQUENCE_ERR",
			"trace_id", traceID,
			"src_ip", srcIP,
			"acct_session_id", attrs.AcctSessionID,
			"reason", "no_start_received",
		)
		// Start相当の処理としてマーク
		_ = p.duplicateDetector.MarkAsStart(ctx, attrs.AcctSessionID)
	}

	// 3. セッション更新
	sessionUUID := attrs.ClassUUID
	if sessionUUID != "" {
		err = p.sessionManager.UpdateOnInterim(ctx, sessionUUID, &session.SessionInterimData{
			NasIP:        srcIP,
			ClientIP:     attrs.FramedIPAddress,
			InputOctets:  int64(attrs.InputOctets),
			OutputOctets: int64(attrs.OutputOctets),
		})
		if err != nil {
			slog.Error("session update failed",
				"event_id", "DB_WRITE_ERR",
				"trace_id", traceID,
				"error", err.Error(),
			)
		}
	}

	// 4. ログ出力
	imsi := p.identifierResolver.ResolveIMSI(ctx, sessionUUID, attrs.UserName, attrs.ClassUUID)
	slog.Info("accounting interim",
		"event_id", "ACCT_INTERIM",
		"trace_id", traceID,
		"src_ip", srcIP,
		"imsi", imsi,
		"acct_session_id", attrs.AcctSessionID,
		"input_octets", attrs.InputOctets,
		"output_octets", attrs.OutputOctets,
	)

	return nil
}
