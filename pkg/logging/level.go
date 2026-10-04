package logging

import (
	"log/slog"
	"strings"
)

// ParseLevel はLOG_LEVEL環境変数の値（DEBUG / INFO / WARN / ERROR、大文字小文字を区別しない）を
// slog.Levelに変換する。未知の値や空文字はINFOとして扱う。
func ParseLevel(s string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
