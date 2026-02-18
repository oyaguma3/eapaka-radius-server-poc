// Package main はAuth Server（EAP-AKA/AKA' RADIUS認証サーバー）のエントリーポイント。
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/engine"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/policy"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/server"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/session"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/vector"
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
	})).With("app", "auth-server")
	slog.SetDefault(logger)

	slog.Info("auth-server起動開始",
		"listen_addr", cfg.ListenAddr,
		"vector_api_url", cfg.VectorAPIURL,
		"network_name", cfg.NetworkName,
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

	// 4. Vector Gatewayクライアント初期化
	vectorClient := vector.NewClient(cfg)

	// 5. Store/Session層生成
	clientStore := store.NewClientStore(valkeyClient)
	policyStore := store.NewPolicyStore(valkeyClient)
	ctxStore := session.NewContextStore(valkeyClient)
	sessStore := session.NewSessionStore(valkeyClient)

	// 6. ポリシー評価器
	evaluator := policy.NewEvaluator()

	// 7. EAPエンジン
	eapEngine := engine.NewEngine(vectorClient, ctxStore, sessStore, policyStore, evaluator, cfg)

	// 8. RADIUS Secret解決
	secretSource := server.NewSecretSource(clientStore, cfg.RadiusSecret)

	// 9. RADIUSハンドラ
	handler := server.NewHandler(eapEngine)

	// 10. UDPサーバー
	srv := server.NewServer(cfg.ListenAddr, handler, secretSource)

	// 11. サーバー起動（goroutine）
	go func() {
		slog.Info("RADIUSサーバー起動", "addr", cfg.ListenAddr)
		if err := srv.ListenAndServe(); err != nil {
			slog.Error("サーバーエラー", "error", err)
		}
	}()

	// 12. シグナル待機 → Graceful Shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	sig := <-sigCh
	slog.Info("シグナル受信、シャットダウン開始", "signal", sig)

	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("シャットダウンエラー", "error", err)
	}

	slog.Info("auth-server停止完了")
}
