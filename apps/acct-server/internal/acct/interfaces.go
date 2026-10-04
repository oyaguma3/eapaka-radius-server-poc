package acct

import (
	"context"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
)

// AccountingProcessor はAccounting処理のインターフェース。
// 各処理は内部のエラー（Valkey障害等）をログに記録して継続し、呼び出し元には返さない
// （RADIUS的には常にAccounting-Responseを返すため）。
type AccountingProcessor interface {
	// ProcessStart はAcct-Start処理を行う
	ProcessStart(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string)
	// ProcessInterim はAcct-Interim処理を行う
	ProcessInterim(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string)
	// ProcessStop はAcct-Stop処理を行う
	ProcessStop(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string)
	// ProcessOn はAccounting-On（NAS起動通知）を処理する
	ProcessOn(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string)
	// ProcessOff はAccounting-Off（NASシャットダウン通知）を処理する
	ProcessOff(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string)
}

// DuplicateDetector は重複・順序異常検出のインターフェース
type DuplicateDetector interface {
	// CheckAndMarkStart はStartの重複をチェックし、未登録ならマークする
	CheckAndMarkStart(ctx context.Context, acctSessionID string) (isDuplicate bool, err error)
	// CheckInterim はInterimの重複と順序異常を判定し、重複でなければ受信値を記録する
	CheckInterim(ctx context.Context, acctSessionID string, input, output uint32) (InterimCheckResult, error)
	// CheckStopDuplicate はStopの重複をチェックする
	CheckStopDuplicate(ctx context.Context, acctSessionID string) (isDuplicate bool, err error)
	// MarkAsStopped はStopとしてマークする
	MarkAsStopped(ctx context.Context, acctSessionID string) error
}

// InterimCheckResult はInterim受信時の重複・順序判定の結果
type InterimCheckResult struct {
	// Duplicate は直前に受信したInterimと同一値（重複）かどうか
	Duplicate bool
	// SequenceReason は順序異常の理由（正常なら空）
	//   - "no_start_received": Start（およびInterim）を受信していない
	//   - "interim_after_stop": Stop受信後のInterim
	SequenceReason string
}
