package logging

import "strings"

// MaskIMSI はIMSI文字列をマスキングする（D-04準拠）。
// 先頭6文字 + マスク文字('*') + 末尾1文字の形式で出力する。
// enabled=falseまたは文字列長が7以下の場合はそのまま返す。
func MaskIMSI(imsi string, enabled bool) string {
	if !enabled || len(imsi) <= 7 {
		return imsi
	}
	return imsi[:6] + strings.Repeat("*", len(imsi)-7) + imsi[len(imsi)-1:]
}
