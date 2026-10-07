// Package main は Provisioning API のエントリーポイント（D-13）。
// 加入者・RADIUSクライアント・認可ポリシーの CRUD を、mTLS で保護した REST API として提供する。
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/auth"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/handler"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/server"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/service"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/valkey"
)

// version は provisioning-api のバージョン（/status の version）。
// ビルド時に -ldflags "-X main.version=..." で上書きできる。
var version = "0.1.0"

func main() {
	startedAt := time.Now()

	// アプリケーションログと監査ログは同じ標準出力に出すため、1行ずつ書き込むよう排他する
	out := &lockedWriter{w: os.Stdout}

	// 1. 設定読み込み
	cfg, err := config.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(out, nil)).With("app", "provisioning-api").
			Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// 2. ロガー初期化
	log := slog.New(slog.NewJSONHandler(out, &slog.HandlerOptions{Level: logging.ParseLevel(cfg.LogLevel)})).
		With("app", "provisioning-api")
	slog.SetDefault(log)

	log.Info("starting provisioning-api",
		"version", version,
		"listen_addr", cfg.ListenAddr,
		"log_level", cfg.LogLevel,
		"node_name", cfg.NodeName,
		"admin_clients", len(cfg.AdminClients),
	)

	// 3. サーバー証明書の読み込み
	cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
	if err != nil {
		log.Error("failed to load server certificate", "cert", cfg.TLSCert, "key", cfg.TLSKey, "error", err)
		os.Exit(1)
	}

	// 4. Valkey接続
	rdb, err := valkey.NewClient(valkey.DefaultOptions().WithAddr(cfg.RedisAddr()).WithPassword(cfg.RedisPass))
	if err != nil {
		log.Error("failed to connect to Valkey", "addr", cfg.RedisAddr(), "error", err)
		os.Exit(1)
	}
	defer func() { _ = rdb.Close() }()
	log.Info("connected to Valkey", "addr", cfg.RedisAddr())

	// 5. 依存オブジェクト生成
	svc := service.New(rdb, audit.NewLogger(out))
	h := handler.New(svc, log, version, cfg.NodeName, startedAt)
	gin.SetMode(cfg.GinMode)
	engine := server.NewEngine(h, cfg.AdminClients, log, logging.NewMasker(cfg.LogMaskIMSI))
	verifier := &auth.Verifier{Clients: cfg.AdminClients, Log: log}
	srv := server.New(cfg.ListenAddr, engine, verifier.TLSConfig(cert), log)

	// 6. サーバー起動
	go func() {
		if err := srv.Run(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// 7. シグナル待機と Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("server shutdown error", "error", err)
	}
	log.Info("server stopped")
}

// lockedWriter は書き込みを排他する io.Writer。
type lockedWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
