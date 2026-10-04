// Package logging はログ関連のユーティリティを提供する。
package logging

import "strings"

// MaskIMSI はIMSIをマスキングする。
// D-04準拠: 先頭6桁 + マスク + 末尾1桁
// 例: 440101234567890 → 440101********0
// enabled=false の場合はマスキングせずにそのまま返す。
func MaskIMSI(imsi string, enabled bool) string {
	if !enabled {
		return imsi
	}
	return MaskPartial(imsi, 6, 1, '*')
}

// MaskPartial は文字列の一部をマスキングする。
// keepPrefix: 先頭から保持する文字数
// keepSuffix: 末尾から保持する文字数
// maskChar: マスキングに使用する文字
func MaskPartial(s string, keepPrefix, keepSuffix int, maskChar rune) string {
	runes := []rune(s)
	length := len(runes)

	// 文字列が短すぎる場合はそのまま返す
	if length <= keepPrefix+keepSuffix {
		return s
	}

	result := make([]rune, length)

	// 先頭部分をコピー
	for i := 0; i < keepPrefix; i++ {
		result[i] = runes[i]
	}

	// 中間部分をマスク
	for i := keepPrefix; i < length-keepSuffix; i++ {
		result[i] = maskChar
	}

	// 末尾部分をコピー
	for i := length - keepSuffix; i < length; i++ {
		result[i] = runes[i]
	}

	return string(result)
}

// MaskUserName はEAP Identity形式のユーザー名（User-Name属性）をマスキングする。
// "@" より前（ローカル部）だけを対象とし、realmはそのまま残す。
//   - 種別1文字＋IMSI 15桁（例: 0440101234567890）: 種別文字を残し、IMSI部分にMaskIMSIを適用
//   - IMSI 15桁のみ: MaskIMSIを適用
//   - それ以外（仮名・不正形式）: 先頭7文字と末尾1文字を残してマスク
//
// enabled=false の場合はマスキングせずにそのまま返す。
func MaskUserName(userName string, enabled bool) string {
	if !enabled {
		return userName
	}

	local, realm := userName, ""
	if i := strings.Index(userName, "@"); i >= 0 {
		local, realm = userName[:i], userName[i:]
	}

	switch {
	case len(local) == 16 && isDigits(local):
		local = local[:1] + MaskIMSI(local[1:], true)
	case len(local) == 15 && isDigits(local):
		local = MaskIMSI(local, true)
	default:
		local = MaskPartial(local, 7, 1, '*')
	}
	return local + realm
}

// isDigits は文字列が数字のみで構成されているかを返す。
func isDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Masker はマスキング設定を保持する構造体。
type Masker struct {
	enabled bool
}

// NewMasker は新しいMaskerを生成する。
func NewMasker(enabled bool) *Masker {
	return &Masker{enabled: enabled}
}

// IMSI はIMSIをマスキングする。
func (m *Masker) IMSI(imsi string) string {
	return MaskIMSI(imsi, m.enabled)
}

// UserName はEAP Identity形式のユーザー名をマスキングする。
func (m *Masker) UserName(userName string) string {
	return MaskUserName(userName, m.enabled)
}

// IsEnabled はマスキングが有効かどうかを返す。
func (m *Masker) IsEnabled() bool {
	return m.enabled
}
