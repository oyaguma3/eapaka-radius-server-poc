package acct

import (
	"context"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
)

// AccountingProcessor はAccounting処理のインターフェース
type AccountingProcessor interface {
	// ProcessStart はAcct-Start処理を行う
	ProcessStart(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error
	// ProcessInterim はAcct-Interim処理を行う
	ProcessInterim(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error
	// ProcessStop はAcct-Stop処理を行う
	ProcessStop(ctx context.Context, attrs *radius.AccountingAttributes, srcIP, traceID string) error
}

// DuplicateDetector は重複・順序異常検出のインターフェース
type DuplicateDetector interface {
	// CheckAndMarkStart はStartの重複をチェックし、未登録ならマークする
	CheckAndMarkStart(ctx context.Context, acctSessionID string) (isDuplicate bool, err error)
	// CheckInterimDuplicate はInterimの重複をチェックする
	CheckInterimDuplicate(ctx context.Context, acctSessionID string, input, output uint32) (isDuplicate bool, err error)
	// CheckStopDuplicate はStopの重複をチェックする
	CheckStopDuplicate(ctx context.Context, acctSessionID string) (isDuplicate bool, err error)
	// HasSeenStart はStartを受信済みかチェックする
	HasSeenStart(ctx context.Context, acctSessionID string) (bool, error)
	// MarkAsStart はStartとしてマークする
	MarkAsStart(ctx context.Context, acctSessionID string) error
	// MarkAsStopped はStopとしてマークする
	MarkAsStopped(ctx context.Context, acctSessionID string) error
}
