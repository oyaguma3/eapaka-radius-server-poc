package logging

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNewRADIUSLibraryLogger(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		wantLevel string // 空なら出力されないこと（ハンドラーのレベルが INFO のため DEBUG は出ない）
	}{
		{"解析エラーはWARN", "radius: unable to parse packet: radius: packet not at least 20 bytes", "WARN"},
		{"受信エラーはWARN", "radius: could not read packet: some error", "WARN"},
		{"シークレットが空はDEBUG", "radius: empty secret returned from secret source", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})))
			t.Cleanup(func() { slog.SetDefault(prev) })

			NewRADIUSLibraryLogger().Printf("%s", tt.line)

			out := buf.String()
			if tt.wantLevel == "" {
				if out != "" {
					t.Errorf("expected no output at INFO level, got: %s", out)
				}
				return
			}
			for _, want := range []string{
				`"level":"` + tt.wantLevel + `"`,
				`"event_id":"RADIUS_LIB_ERR"`,
				`"error":"` + tt.line + `"`,
			} {
				if !strings.Contains(out, want) {
					t.Errorf("log does not contain %s: %s", want, out)
				}
			}
		})
	}
}

func TestNewRADIUSLibraryLogger_DebugLevel(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	NewRADIUSLibraryLogger().Printf("radius: empty secret returned from secret source")

	if out := buf.String(); !strings.Contains(out, `"level":"DEBUG"`) || !strings.Contains(out, `"event_id":"RADIUS_LIB_ERR"`) {
		t.Errorf("expected DEBUG log, got: %s", out)
	}
}
