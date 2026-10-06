package format

// OrDash は空文字列を "-" に置き換える（値のない項目の表示用）。
func OrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
