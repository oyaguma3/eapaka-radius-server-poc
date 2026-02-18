package acct

import (
	"context"
	"log/slog"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
)

// ProcessStop はAcct-Stop処理を行う。
func (p *Processor) ProcessStop(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error {
	// 1. Stop重複チェック
	isDuplicate, err := p.duplicateDetector.CheckStopDuplicate(ctx, attrs.AcctSessionID)
	if err != nil {
		// Valkey障害時は処理継続
		slog.Error("duplicate check failed",
			"event_id", "VALKEY_CONN_ERR",
			"trace_id", traceID,
			"error", err.Error(),
		)
	}
	if isDuplicate {
		// Stop重複時はログ出力なしで処理終了
		return nil
	}

	// 2. Stopとしてマーク
	if err := p.duplicateDetector.MarkAsStopped(ctx, attrs.AcctSessionID); err != nil {
		slog.Error("duplicate mark failed",
			"event_id", "DB_WRITE_ERR",
			"trace_id", traceID,
			"error", err.Error(),
		)
	}

	// 3. セッション削除
	sessionUUID := attrs.ClassUUID
	var imsiFromSession string
	if sessionUUID != "" {
		// IMSI取得（削除前に）
		sess, err := p.sessionManager.Get(ctx, sessionUUID)
		if err == nil && sess != nil {
			imsiFromSession = sess.IMSI
		}

		// セッション削除
		if err := p.sessionManager.Delete(ctx, sessionUUID); err != nil {
			slog.Error("session delete failed",
				"event_id", "DB_WRITE_ERR",
				"trace_id", traceID,
				"error", err.Error(),
			)
		}

		// インデックス削除
		if imsiFromSession != "" {
			if err := p.sessionManager.RemoveUserIndex(ctx, imsiFromSession, sessionUUID); err != nil {
				slog.Error("index delete failed",
					"event_id", "DB_WRITE_ERR",
					"trace_id", traceID,
					"error", err.Error(),
				)
			}
		}
	}

	// 4. ログ出力
	imsi := p.identifierResolver.ResolveIMSI(ctx, sessionUUID, attrs.UserName, attrs.ClassUUID)
	slog.Info("accounting stop",
		"event_id", "ACCT_STOP",
		"trace_id", traceID,
		"src_ip", srcIP,
		"imsi", imsi,
		"acct_session_id", attrs.AcctSessionID,
		"input_octets", attrs.InputOctets,
		"output_octets", attrs.OutputOctets,
		"session_time", attrs.SessionTime,
	)

	return nil
}
