package main

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/backend"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
)

func TestAKAOnlyTransport(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"", "disabled"},
		{"https://aka-only-server:8443", "mtls"},
		{"http://aka-only-server:8080", "plain"},
	}
	for _, tt := range tests {
		cfg := &config.Config{AKAOnlyURL: tt.url}
		if got := akaOnlyTransport(cfg); got != tt.want {
			t.Errorf("akaOnlyTransport(%q) = %q, want %q", tt.url, got, tt.want)
		}
	}
}

func TestWarnBackendConfig(t *testing.T) {
	tests := []struct {
		name     string
		cfg      config.Config
		plmnMap  map[string]string
		wantWarn []string
		noWarn   []string
	}{
		{
			name:     "01 referenced but not configured",
			cfg:      config.Config{Mode: "gateway"},
			plmnMap:  map[string]string{"44010": "01"},
			wantWarn: []string{"PLMN map refers to a backend that is not configured", `"backend_id":"01"`},
		},
		{
			name:     "plain HTTP",
			cfg:      config.Config{Mode: "gateway", AKAOnlyURL: "http://aka-only-server:8080", AKAOnlyTimeout: time.Second},
			plmnMap:  map[string]string{"44010": "01"},
			wantWarn: []string{"plain HTTP"},
			noWarn:   []string{"not configured"},
		},
		{
			name:    "passthrough ignores PLMN map",
			cfg:     config.Config{Mode: "passthrough"},
			plmnMap: map[string]string{"44010": "01"},
			noWarn:  []string{"not configured"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			defer slog.SetDefault(prev)

			cfg := tt.cfg
			cfg.InternalURL = "http://localhost:8080"
			cfg.InternalTimeout = time.Second
			reg, err := backend.NewRegistry(&cfg)
			if err != nil {
				t.Fatalf("NewRegistry() error = %v", err)
			}
			warnBackendConfig(&cfg, tt.plmnMap, reg)

			logs := buf.String()
			for _, s := range tt.wantWarn {
				if !strings.Contains(logs, s) {
					t.Errorf("log does not contain %q: %s", s, logs)
				}
			}
			for _, s := range tt.noWarn {
				if strings.Contains(logs, s) {
					t.Errorf("log unexpectedly contains %q: %s", s, logs)
				}
			}
		})
	}
}

func TestInitLogger(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	tests := []struct {
		level string
		want  slog.Level
	}{
		{"DEBUG", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"WARN", slog.LevelWarn},
		{"ERROR", slog.LevelError},
	}
	for _, tt := range tests {
		initLogger(&config.Config{LogLevel: tt.level})
		ctx := context.Background()
		if !slog.Default().Enabled(ctx, tt.want) {
			t.Errorf("level %q: %v should be enabled", tt.level, tt.want)
		}
		if tt.want > slog.LevelDebug && slog.Default().Enabled(ctx, tt.want-1) {
			t.Errorf("level %q: below %v should be disabled", tt.level, tt.want)
		}
	}
}
