// Package format はフォーマットユーティリティを提供する。
package format

import "fmt"

// Bytes はバイト数を人間が読みやすい形式にフォーマットする。
// 例: 1024 -> "1.00 KB", 1048576 -> "1.00 MB"
func Bytes(bytes int64) string {
	const (
		_          = iota
		kb float64 = 1 << (10 * iota)
		mb
		gb
		tb
	)

	b := float64(bytes)

	switch {
	case b >= tb:
		return fmt.Sprintf("%.2f TB", b/tb)
	case b >= gb:
		return fmt.Sprintf("%.2f GB", b/gb)
	case b >= mb:
		return fmt.Sprintf("%.2f MB", b/mb)
	case b >= kb:
		return fmt.Sprintf("%.2f KB", b/kb)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// BytesShort はバイト数を短い形式にフォーマットする。
// 例: 1024 -> "1K", 1048576 -> "1M"
func BytesShort(bytes int64) string {
	const (
		_          = iota
		kb float64 = 1 << (10 * iota)
		mb
		gb
		tb
	)

	b := float64(bytes)

	switch {
	case b >= tb:
		return fmt.Sprintf("%.1fT", b/tb)
	case b >= gb:
		return fmt.Sprintf("%.1fG", b/gb)
	case b >= mb:
		return fmt.Sprintf("%.1fM", b/mb)
	case b >= kb:
		return fmt.Sprintf("%.1fK", b/kb)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}
