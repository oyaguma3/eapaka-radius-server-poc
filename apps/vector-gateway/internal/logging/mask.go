// Package logging はログ出力に関するユーティリティを提供する。
package logging

// MaskIMSI はログ出力用にIMSIをマスクする。
// enabled が false の場合、またはIMSIが6文字以下の場合はそのまま返す。
// それ以外は先頭6文字と末尾1文字を残してマスクする。
func MaskIMSI(imsi string, enabled bool) string {
	if !enabled {
		return imsi
	}
	if len(imsi) <= 6 {
		return imsi
	}
	return imsi[:6] + "********" + imsi[len(imsi)-1:]
}
