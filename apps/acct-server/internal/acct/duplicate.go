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

// CheckInterimDuplicate はInterimの重複をチェックする。
// 同一のinput/output値の場合は重複とみなす。
func (d *duplicateDetector) CheckInterimDuplicate(ctx context.Context, acctSessionID string, input, output uint32) (bool, error) {
	currentVal := fmt.Sprintf("interim:%d:%d", input, output)

	val, err := d.dupStore.Get(ctx, acctSessionID)
	if err != nil {
		return false, err
	}

	if val == currentVal {
		return true, nil
	}

	// 値を更新
	if err := d.dupStore.Set(ctx, acctSessionID, currentVal); err != nil {
		return false, err
	}
	return false, nil
}

// CheckStopDuplicate はStopの重複をチェックする。
func (d *duplicateDetector) CheckStopDuplicate(ctx context.Context, acctSessionID string) (bool, error) {
	val, err := d.dupStore.Get(ctx, acctSessionID)
	if err != nil {
		return false, err
	}
	return val == "stop", nil
}

// HasSeenStart はStartを受信済みかチェックする。
func (d *duplicateDetector) HasSeenStart(ctx context.Context, acctSessionID string) (bool, error) {
	val, err := d.dupStore.Get(ctx, acctSessionID)
	if err != nil {
		return false, err
	}
	return val == "start" || strings.HasPrefix(val, "interim:"), nil
}

// MarkAsStart はStartとしてマークする（StartなしInterim受信時に使用）。
func (d *duplicateDetector) MarkAsStart(ctx context.Context, acctSessionID string) error {
	return d.dupStore.Set(ctx, acctSessionID, "start")
}

// MarkAsStopped はStopとしてマークする。
func (d *duplicateDetector) MarkAsStopped(ctx context.Context, acctSessionID string) error {
	return d.dupStore.Set(ctx, acctSessionID, "stop")
}
