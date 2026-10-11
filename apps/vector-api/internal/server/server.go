// Package server はHTTPサーバーの管理を提供する。
package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/handler"
)

// HTTP サーバーの時間の上限。
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
)

// Server はHTTPサーバーを管理する。
type Server struct {
	engine *gin.Engine
	server *http.Server
	cfg    *config.Config
}

// New は新しいServerを生成する。
func New(cfg *config.Config, h *handler.VectorHandler) *Server {
	// Ginモード設定
	gin.SetMode(cfg.GinMode)

	engine := gin.New()

	// ミドルウェア登録
	engine.Use(TraceIDMiddleware())
	engine.Use(LoggingMiddleware())
	engine.Use(RecoveryMiddleware())

	// ルーティング
	SetupRouter(engine, h)

	return &Server{
		engine: engine,
		server: &http.Server{
			Addr:    cfg.ListenAddr,
			Handler: engine,
			// 遅いクライアントや切れない接続でゴルーチンが溜まらないよう、上限を設ける（provisioning-api と同じ値）。
			// 処理の時間はバックエンドへの呼び出しの上限（数秒）に収まるので、書き出しの 30 秒で足りる
			ReadHeaderTimeout: readHeaderTimeout,
			ReadTimeout:       readTimeout,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       idleTimeout,
		},
		cfg: cfg,
	}
}

// Run はサーバーを起動する。
func (s *Server) Run() error {
	slog.Info("starting server", "addr", s.cfg.ListenAddr)
	return s.server.ListenAndServe()
}

// Shutdown はサーバーをシャットダウンする。
func (s *Server) Shutdown(ctx context.Context) error {
	slog.Info("shutting down server")
	return s.server.Shutdown(ctx)
}
