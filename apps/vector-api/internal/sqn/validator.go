package sqn

import "fmt"

const (
	// Delta は3GPP TS 33.102 C.3.2 Profile 2で定義されるSQN許容範囲
	// 2^28 = 268,435,456
	Delta = 1 << 28
)

// Validator はSQNの検証を行う。
type Validator struct{}

// NewValidator は新しいValidatorを生成する。
func NewValidator() *Validator {
	return &Validator{}
}

// ValidateResyncSQN は再同期時のSQN妥当性を検証する。
//
// 検証条件（3GPP TS 33.102 C.3.2 Profile 2）:
// 1. SQN_MS > SQN_HE（端末のSQNがネットワークより進んでいる）
// 2. SQN_MS - SQN_HE <= Δ（差がデルタ以内）
func (v *Validator) ValidateResyncSQN(sqnMS, sqnHE uint64) error {
	// 条件1: SQN_MS > SQN_HE
	if sqnMS <= sqnHE {
		return fmt.Errorf("SQN_MS (%d) must be greater than SQN_HE (%d)", sqnMS, sqnHE)
	}

	// 条件2: 差がΔ以内
	diff := sqnMS - sqnHE
	if diff > Delta {
		return fmt.Errorf("SQN difference exceeds delta: %d > %d", diff, Delta)
	}

	return nil
}

// ComputeResyncSQN は再同期後の新しいSQNを計算する。
// 端末のSQN_MSを基準に、SEQを+1する。
func (v *Validator) ComputeResyncSQN(sqnMS uint64) (uint64, error) {
	newSQN := sqnMS + IncrementStep

	if newSQN > MaxSQN {
		return 0, fmt.Errorf("SQN overflow after resync")
	}

	return newSQN, nil
}
