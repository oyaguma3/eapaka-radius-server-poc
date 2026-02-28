// Package logging はログ関連のユーティリティを提供する。
package logging

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

// IsEnabled はマスキングが有効かどうかを返す。
func (m *Masker) IsEnabled() bool {
	return m.enabled
}
