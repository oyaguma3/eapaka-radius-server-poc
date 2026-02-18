package policy

import "context"

// PolicyStore はポリシーデータへのアクセスを定義する。
type PolicyStore interface {
	// GetPolicy は指定されたIMSIに対応するポリシーを取得する。
	GetPolicy(ctx context.Context, imsi string) (*Policy, error)
}

// Evaluator はポリシー評価エンジンを定義する。
type Evaluator interface {
	// Evaluate はポリシーをNAS-IDとSSIDで評価し、結果を返す。
	Evaluate(policy *Policy, nasID string, ssid string) *EvaluationResult
}
