// Package main はVector Gatewayのエントリーポイント。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/backend"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/handler"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/router"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/server"
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
	)

	// 4. バックエンドレジストリ
	registry := backend.NewRegistry(cfg)

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

// initLogger はロガーを初期化する。
func initLogger(cfg *config.Config) {
	level := slog.LevelInfo
	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		level = slog.LevelDebug
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	h := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(h).With("app", "vector-gateway")
	slog.SetDefault(logger)
}
