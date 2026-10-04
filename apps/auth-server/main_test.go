package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestWarnFallbackSecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		wantWarn bool
	}{
		{"設定ありならWARNを出す", "testing123", true},
		{"空なら出さない", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(prev) })

			warnFallbackSecret(tt.secret)

			out := buf.String()
			if got := strings.Contains(out, `"level":"WARN"`); got != tt.wantWarn {
				t.Errorf("WARN logged = %v, want %v: %s", got, tt.wantWarn, out)
			}
			// シークレットの値はログに出さない
			if tt.secret != "" && strings.Contains(out, tt.secret) {
				t.Errorf("log must not contain the secret: %s", out)
			}
		})
	}
}
