// Package sqn はSQN（シーケンス番号）の管理機能を提供する。
package sqn

import (
	"fmt"
	"strconv"
)

const (
	// MaxSQN は48bit SQNの最大値
	MaxSQN = (1 << 48) - 1

	// IncrementStep はSQNインクリメント時の加算値
	// SQN = SEQ(43bit) || IND(5bit) なので、SEQ+1 = SQN+32
	IncrementStep = 32
)

// Manager はSQN管理を行う。
type Manager struct{}

// NewManager は新しいManagerを生成する。
func NewManager() *Manager {
	return &Manager{}
}

// Increment はSQNをインクリメントする。
// IND部分を固定したまま、SEQ部分のみ+1する。
// 実装上は SQN + 32 で簡略化。
func (m *Manager) Increment(currentSQN uint64) (uint64, error) {
	newSQN := currentSQN + IncrementStep

	// 48bit上限チェック（SEQオーバーフロー）
	if newSQN > MaxSQN {
		return 0, fmt.Errorf("SQN overflow: SEQ reached maximum value")
	}

	return newSQN, nil
}

// GetSEQ はSQNからSEQ部分を抽出する。
func (m *Manager) GetSEQ(sqn uint64) uint64 {
	return sqn >> 5
}

// GetIND はSQNからIND部分を抽出する。
func (m *Manager) GetIND(sqn uint64) uint8 {
	return uint8(sqn & 0x1F)
}

// FormatHex はSQNを12桁Hex文字列に変換する。
func (m *Manager) FormatHex(sqn uint64) string {
	return fmt.Sprintf("%012x", sqn)
}

// ParseHex は12桁Hex文字列をSQNに変換する。
func (m *Manager) ParseHex(s string) (uint64, error) {
	if len(s) != 12 {
		return 0, fmt.Errorf("invalid SQN hex length: expected 12, got %d", len(s))
	}
	return strconv.ParseUint(s, 16, 48)
}
