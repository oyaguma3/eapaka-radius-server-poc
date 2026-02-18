package policy

import "strings"

// evaluator はEvaluatorインターフェースの実装。
type evaluator struct{}

// NewEvaluator は新しいEvaluatorを生成する。
func NewEvaluator() Evaluator {
	return &evaluator{}
}

// Evaluate はポリシーをNAS-IDとSSIDで評価し、結果を返す。
// ルール配列を順次評価し、最初に一致したルールを適用する。
// 一致するルールがない場合はdefault値に基づいて判定する。
func (e *evaluator) Evaluate(p *Policy, nasID string, ssid string) *EvaluationResult {
	for i := range p.Rules {
		rule := &p.Rules[i]

		// NAS-ID完全一致（大文字小文字区別）
		if rule.NasID != nasID {
			continue
		}

		// AllowedSSIDsチェック
		if !matchSSID(rule.AllowedSSIDs, ssid) {
			continue
		}

		return &EvaluationResult{
			Allowed:     true,
			MatchedRule: rule,
		}
	}

	// 一致するルールなし → default判定
	if p.Default == "allow" {
		return &EvaluationResult{
			Allowed: true,
		}
	}

	return &EvaluationResult{
		Allowed:    false,
		DenyReason: "no matching rule and default is deny",
	}
}

// matchSSID はSSIDがAllowedSSIDsリストに一致するかを判定する。
// ["*"]はワイルドカードとして全SSIDに一致する。
// SSID比較は大文字小文字を区別しない。
func matchSSID(allowedSSIDs []string, ssid string) bool {
	for _, allowed := range allowedSSIDs {
		if allowed == "*" {
			return true
		}
		if strings.EqualFold(allowed, ssid) {
			return true
		}
	}
	return false
}

// ExtractSSID はCalled-Station-ID属性からSSID部分を抽出する。
// 形式: "AA-BB-CC-DD-EE-FF:SSID_NAME" → "SSID_NAME"
// コロンがない場合は全体をSSIDとして返却する。
func ExtractSSID(calledStationID string) string {
	idx := strings.Index(calledStationID, ":")
	if idx < 0 {
		return calledStationID
	}
	return calledStationID[idx+1:]
}
