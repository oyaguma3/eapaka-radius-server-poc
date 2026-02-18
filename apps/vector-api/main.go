// Package main はVector APIのエントリーポイント。
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

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/handler"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/milenage"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/server"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/sqn"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/testmode"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/usecase"
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

	slog.Info("starting vector-api",
		"listen_addr", cfg.ListenAddr,
		"log_level", cfg.LogLevel,
		"test_mode", cfg.TestVectorEnabled,
	)

	// 3. Valkey接続
	valkeyClient, err := store.NewValkeyClient(cfg)
	if err != nil {
		slog.Error("failed to connect to Valkey", "error", err)
		os.Exit(1)
	}
	defer valkeyClient.Close()

	slog.Info("connected to Valkey", "addr", cfg.RedisAddr())

	// 4. 依存オブジェクト生成
	subscriberStore := store.NewSubscriberStore(valkeyClient)
	calculator := milenage.NewCalculator()
	resyncProcessor := milenage.NewResyncProcessor()
	sqnManager := sqn.NewManager()
	sqnValidator := sqn.NewValidator()

	// テストモード設定
	var testVectorProvider usecase.TestVectorProvider
	if cfg.TestVectorEnabled {
		testVectorProvider = testmode.NewTestVectorProvider(cfg.TestVectorIMSIPrefix)
		slog.Warn("test vector mode enabled",
			"imsi_prefix", cfg.TestVectorIMSIPrefix,
		)
	}

	// ユースケース
	vectorUseCase := usecase.NewVectorUseCase(
		subscriberStore,
		calculator,
		sqnManager,
		sqnValidator,
		resyncProcessor,
		testVectorProvider,
		cfg,
	)

	// ハンドラー
	vectorHandler := handler.NewVectorHandler(vectorUseCase, cfg)

	// 5. サーバー起動
	srv := server.New(cfg, vectorHandler)

	// 6. Graceful Shutdown設定
	go func() {
		if err := srv.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 7. シグナル待機
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

	handler := slog.NewJSONHandler(os.Stdout, opts)
	logger := slog.New(handler).With("app", "vector-api")
	slog.SetDefault(logger)
}
