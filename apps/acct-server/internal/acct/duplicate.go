package acct

import (
	"context"
	"fmt"
	"strings"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/store"
)

// duplicateDetector はDuplicateDetectorインターフェースの実装。
type duplicateDetector struct {
	dupStore store.DuplicateStore
}

// NewDuplicateDetector は新しいDuplicateDetectorを生成する。
func NewDuplicateDetector(ds store.DuplicateStore) DuplicateDetector {
	return &duplicateDetector{dupStore: ds}
}

// CheckAndMarkStart はStartの重複をチェックし、未登録ならマークする。
func (d *duplicateDetector) CheckAndMarkStart(ctx context.Context, acctSessionID string) (bool, error) {
	val, err := d.dupStore.Get(ctx, acctSessionID)
	if err != nil {
		return false, err
	}

	if val == "" {
		// 新規：マークして継続
		if err := d.dupStore.Set(ctx, acctSessionID, "start"); err != nil {
			return false, err
		}
		return false, nil
	}

	// Stop後のStart検出（順序異常だが新規セッションとして扱う）
	if val == "stop" {
		if err := d.dupStore.Set(ctx, acctSessionID, "start"); err != nil {
			return false, err
		}
		return false, &SequenceError{Reason: "start_after_stop"}
	}

	// Start重複
	if val == "start" || strings.HasPrefix(val, "interim:") {
		return true, nil
	}

	return false, nil
}

// 順序異常の理由
const (
	seqReasonNoStart          = "no_start_received"
	seqReasonInterimAfterStop = "interim_after_stop"
)

// CheckInterim はInterimの重複と順序異常を判定する。
// 1回のGetで直前の状態を取得してから判定するため、判定前に自身の書き込みで状態が変わることはない。
// 同一のinput/output値が直前に記録されていれば重複とし、記録を変更しない。
// 重複でなければ（順序異常の場合も含め）受信値を記録する。
func (d *duplicateDetector) CheckInterim(ctx context.Context, acctSessionID string, input, output uint32) (InterimCheckResult, error) {
	var result InterimCheckResult
	currentVal := fmt.Sprintf("interim:%d:%d", input, output)

	val, err := d.dupStore.Get(ctx, acctSessionID)
	if err != nil {
		return result, err
	}

	switch val {
	case currentVal:
		result.Duplicate = true
		return result, nil
	case "":
		result.SequenceReason = seqReasonNoStart
	case "stop":
		result.SequenceReason = seqReasonInterimAfterStop
	}

	if err := d.dupStore.Set(ctx, acctSessionID, currentVal); err != nil {
		return result, err
	}
	return result, nil
}

// CheckStopDuplicate はStopの重複をチェックする。
func (d *duplicateDetector) CheckStopDuplicate(ctx context.Context, acctSessionID string) (bool, error) {
	val, err := d.dupStore.Get(ctx, acctSessionID)
	if err != nil {
		return false, err
	}
	return val == "stop", nil
}

// MarkAsStopped はStopとしてマークする。
func (d *duplicateDetector) MarkAsStopped(ctx context.Context, acctSessionID string) error {
	return d.dupStore.Set(ctx, acctSessionID, "stop")
}
