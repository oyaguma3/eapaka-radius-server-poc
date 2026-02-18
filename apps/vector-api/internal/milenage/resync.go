package milenage

import (
	"crypto/subtle"
	"fmt"

	"github.com/wmnsk/milenage"
)

// ResyncProcessor はAUTS処理を行う。
type ResyncProcessor struct{}

// NewResyncProcessor は新しいResyncProcessorを生成する。
func NewResyncProcessor() *ResyncProcessor {
	return &ResyncProcessor{}
}

// ExtractSQN はAUTSからSQN_MSを抽出する。
// AUTS = (SQN_MS ⊕ AK*) || MAC-S
//
// 処理手順:
// 1. f5*(AK*) を計算
// 2. SQN_MS = (SQN_MS ⊕ AK*) ⊕ AK* で復号
// 3. f1*(MAC-S) を計算して検証
func (r *ResyncProcessor) ExtractSQN(ki, opc, randVal, auts []byte) (uint64, error) {
	if len(auts) != 14 {
		return 0, fmt.Errorf("invalid AUTS length: expected 14, got %d", len(auts))
	}

	// Milenage構造体を作成（SQN=0, AMF=0 でダミー初期化）
	m := milenage.NewWithOPc(ki, opc, randVal, 0, 0)

	// 1. f5*計算（AK*）
	akStar, err := m.F5Star()
	if err != nil {
		return 0, fmt.Errorf("failed to compute f5*: %w", err)
	}

	// 2. SQN_MS復号
	sqnMSXorAKStar := auts[:6]
	sqnMSBytes := make([]byte, 6)
	for i := 0; i < 6; i++ {
		sqnMSBytes[i] = sqnMSXorAKStar[i] ^ akStar[i]
	}

	// 3. MAC-S検証
	macSReceived := auts[6:14]

	// AMFは再同期時は固定値（0x0000）を使用
	amfResync := []byte{0x00, 0x00}

	macSComputed, err := m.F1Star(sqnMSBytes, amfResync)
	if err != nil {
		return 0, fmt.Errorf("failed to compute f1*: %w", err)
	}

	// MAC-S比較（タイミング攻撃対策で定数時間比較）
	if subtle.ConstantTimeCompare(macSReceived, macSComputed) != 1 {
		return 0, fmt.Errorf("MAC-S verification failed")
	}

	return BytesToSQN(sqnMSBytes), nil
}
