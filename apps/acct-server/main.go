// Package main はAcct Server（RADIUS Accountingサーバー）のエントリーポイント。
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/acct"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/server"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/session"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/store"
)

func main() {
	// 1. 環境変数読み込み
	cfg, err := config.Load()
	if err != nil {
		slog.Error("設定読み込み失敗", "error", err)
		os.Exit(1)
	}

	// 2. ロガー初期化（JSON形式、INFO以上）
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})).With("app", "acct-server")
	slog.SetDefault(logger)

	slog.Info("acct-server起動開始",
		"listen_addr", cfg.ListenAddr,
	)

	// 3. Valkeyクライアント初期化
	valkeyClient, err := store.NewValkeyClient(cfg)
	if err != nil {
		slog.Error("Valkey接続失敗",
			"event_id", "VALKEY_CONN_ERR",
			"error", err,
		)
		os.Exit(1)
	}
	defer valkeyClient.Close()

	slog.Info("Valkey接続完了", "addr", cfg.ValkeyAddr())

	// 4. Store層生成
	clientStore := store.NewClientStore(valkeyClient)
	sessionStore := store.NewSessionStore(valkeyClient)
	duplicateStore := store.NewDuplicateStore(valkeyClient)

	// 5. Session層生成
	sessionManager := session.NewManager(sessionStore)
	identifierResolver := session.NewIdentifierResolver(sessionManager, cfg.LogMaskIMSI)

	// 6. Acct層生成
	duplicateDetector := acct.NewDuplicateDetector(duplicateStore)
	processor := acct.NewProcessor(sessionManager, duplicateDetector, identifierResolver)

	// 7. RADIUS Secret解決
	secretSource := server.NewSecretSource(clientStore, cfg.RadiusSecret)

	// 8. RADIUSハンドラ
	handler := server.NewHandler(processor)

	// 9. UDPサーバー
	srv := server.NewServer(cfg.ListenAddr, handler, secretSource)

	// 10. サーバー起動（goroutine）
	go func() {
		slog.Info("RADIUSサーバー起動", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil {
			slog.Error("サーバーエラー", "error", err)
		}
	}()

	// 11. シグナル待機 → Graceful Shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigCh
	slog.Info("シグナル受信、シャットダウン開始", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("シャットダウンエラー", "error", err)
	}

	slog.Info("acct-server停止完了")
}
