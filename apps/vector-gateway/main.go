// Package main はVector Gatewayのエントリーポイント。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/backend"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/handler"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/router"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/server"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
)

func main() {
	// 1. 設定読み込み
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 2. ロガー初期化
	initLogger(cfg)

	// 3. PLMNマップパース
	plmnMap, err := cfg.ParsePLMNMap()
	if err != nil {
		slog.Error("failed to parse PLMN map", "error", err)
		os.Exit(1)
	}

	slog.Info("starting vector-gateway",
		"listen_addr", cfg.ListenAddr,
		"log_level", cfg.LogLevel,
		"mode", cfg.Mode,
		"plmn_map_entries", len(plmnMap),
		"akaonly_enabled", cfg.AKAOnlyEnabled(),
		"akaonly_transport", akaOnlyTransport(cfg),
	)

	// 4. バックエンドレジストリ
	registry, err := backend.NewRegistry(cfg)
	if err != nil {
		slog.Error("failed to initialize backends", "error", err)
		os.Exit(1)
	}
	warnBackendConfig(cfg, plmnMap, registry)

	// 5. ルーター
	r := router.NewRouter(plmnMap, registry, cfg.IsPassthrough())

	// 6. ハンドラー
	vectorHandler := handler.NewVectorHandler(r, cfg)

	// 7. サーバー起動
	srv := server.New(cfg, vectorHandler)

	// 8. Graceful Shutdown設定
	go func() {
		if err := srv.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 9. シグナル待機
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	slog.Info("server stopped")
}

// authServerVectorTimeout は auth-server が Vector Gateway を呼び出すときのタイムアウト
// （apps/auth-server/internal/config.VectorRequestTimeout と同じ値）。
const authServerVectorTimeout = 5 * time.Second

// akaOnlyTransport はaka-only-serverへの接続方式（mtls / plain / disabled）を返す。
func akaOnlyTransport(cfg *config.Config) string {
	switch {
	case !cfg.AKAOnlyEnabled():
		return "disabled"
	case cfg.AKAOnlyUseTLS():
		return "mtls"
	default:
		return "plain"
	}
}

// warnBackendConfig は起動を止めるほどではない設定上の注意をWARNログに出す。
func warnBackendConfig(cfg *config.Config, plmnMap map[string]string, registry *backend.Registry) {
	// バックエンド向けタイムアウトは auth-server の呼び出しタイムアウトより短くする必要がある
	timeouts := []struct {
		name string
		d    time.Duration
		used bool
	}{
		{"VECTOR_GATEWAY_INTERNAL_TIMEOUT", cfg.InternalTimeout, true},
		{"VECTOR_GATEWAY_AKAONLY_TIMEOUT", cfg.AKAOnlyTimeout, cfg.AKAOnlyEnabled()},
	}
	for _, t := range timeouts {
		if t.used && t.d >= authServerVectorTimeout {
			slog.Warn("backend timeout should be shorter than the auth-server timeout; auth-server may time out before receiving 502",
				"setting", t.name,
				"timeout", t.d.String(),
				"auth_server_timeout", authServerVectorTimeout.String(),
			)
		}
	}

	if cfg.AKAOnlyEnabled() && !cfg.AKAOnlyUseTLS() {
		slog.Warn("aka-only-server is connected over plain HTTP; CK/IK are transmitted unencrypted",
			"akaonly_url", cfg.AKAOnlyURL,
		)
	}
	if cfg.IsPassthrough() {
		return
	}
	for plmn, id := range plmnMap {
		if _, err := registry.Get(id); err != nil {
			slog.Warn("PLMN map refers to a backend that is not configured; requests will fail with 501",
				"plmn", plmn,
				"backend_id", id,
			)
		}
	}
}

// initLogger はロガーを初期化する。
func initLogger(cfg *config.Config) {
	opts := &slog.HandlerOptions{
		Level: logging.ParseLevel(cfg.LogLevel),
	}

	h := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(h).With("app", "vector-gateway")
	slog.SetDefault(logger)
}
