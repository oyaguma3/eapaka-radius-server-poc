// Package main はAcct Server（RADIUS Accountingサーバー）のエントリーポイント。
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/acct"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/server"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/session"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
)

// build はビルドの版（git のコミットなど）。起動ログの build に出す。
// イメージのビルドで -ldflags "-X main.build=..." として埋め込む（compose の POC_VERSION。既定 dev）。
var build = "dev"

func main() {
	// 1. 環境変数読み込み
	cfg, err := config.Load()
	if err != nil {
		slog.Error("設定読み込み失敗", "error", err)
		os.Exit(1)
	}

	// 2. ロガー初期化（JSON形式、LOG_LEVEL 以上。既定 INFO）
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logging.ParseLevel(cfg.LogLevel),
	})).With("app", "acct-server")
	slog.SetDefault(logger)

	slog.Info("acct-server起動開始",
		"build", build,
		"listen_addr", cfg.ListenAddr,
		"log_level", cfg.LogLevel,
		"radius_secret_fallback", cfg.RadiusSecret != "",
	)
	warnFallbackSecret(cfg.RadiusSecret)

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

	// 10. サーバー起動、11. シグナル待機 → Graceful Shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	if err := runServer(srv, cfg.ListenAddr, sigCh, config.ShutdownTimeout); err != nil {
		// 待ち受けを開始できない（ポートが使用中など）か、異常終了した。プロセスを終えて、compose の restart で
		// 起動し直させる（終了しないと、待ち受けていないのに pgrep の healthcheck が通ってしまう）
		slog.Error("サーバーエラー", "event_id", "SYS_LISTEN_ERR", "addr", cfg.ListenAddr, "error", err)
		os.Exit(1)
	}

	slog.Info("acct-server停止完了")
}

// radiusServer は runServer が使う RADIUS サーバーの操作（テストで差し替える）。
type radiusServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

// runServer は RADIUS サーバーを起動し、停止のシグナルを受けたらグレースフルに停止して nil を返す。
// シグナルの前にサーバーが終わった（待ち受けの開始の失敗など）ときは、そのエラーを返す。
func runServer(srv radiusServer, addr string, sigCh <-chan os.Signal, shutdownTimeout time.Duration) error {
	errCh := make(chan error, 1)
	go func() {
		slog.Info("RADIUSサーバー起動", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err == nil {
			err = errors.New("RADIUS server stopped unexpectedly")
		}
		return err
	case sig := <-sigCh:
		slog.Info("シグナル受信、シャットダウン開始", "signal", sig)
	}

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Warn("シャットダウンエラー", "error", err)
	}
	return nil
}

// warnFallbackSecret は、RADIUS_SECRET（フォールバックの共有シークレット）が設定されているときにWARNログを出す。
// 設定されていると、クライアント登録（client:{IP}）のない送信元IPからのパケットも
// このシークレットで受け付けるため、インターネットに公開するサーバーでは空にすることを促す。
func warnFallbackSecret(fallbackSecret string) {
	if fallbackSecret == "" {
		return
	}
	slog.Warn("RADIUS_SECRETが設定されているため、クライアント登録のない送信元IPのパケットもフォールバックの共有シークレットで受け付ける（インターネットに公開する場合は空にすること）")
}
