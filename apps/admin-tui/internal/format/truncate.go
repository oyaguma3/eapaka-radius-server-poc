package format

import "unicode/utf8"

// Truncate は文字列を指定した長さに切り詰める。
// 切り詰めた場合は末尾に "..." を付加する。
func Truncate(s string, maxLen int) string {
	if maxLen <= 3 {
		if maxLen <= 0 {
			return ""
		}
		if len(s) <= maxLen {
			return s
		}
		return s[:maxLen]
	}

	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}

	// UTF-8対応で文字数で切り詰め
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	return string(runes[:maxLen-3]) + "..."
}

// TruncateMiddle は文字列の中央を省略して切り詰める。
// 例: "0123456789" -> "012...789" (maxLen=9)
func TruncateMiddle(s string, maxLen int) string {
	if maxLen <= 5 {
		return Truncate(s, maxLen)
	}

	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}

	// 前半と後半の長さを計算（"..."の3文字分を引く）
	remaining := maxLen - 3
	frontLen := remaining / 2
	backLen := remaining - frontLen

	return string(runes[:frontLen]) + "..." + string(runes[len(runes)-backLen:])
}

// PadRight は文字列を右側にパディングする。
func PadRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}

	padding := make([]rune, width-len(runes))
	for i := range padding {
		padding[i] = ' '
	}
	return s + string(padding)
}

// PadLeft は文字列を左側にパディングする。
func PadLeft(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}

	padding := make([]rune, width-len(runes))
	for i := range padding {
		padding[i] = ' '
	}
	return string(padding) + s
}
