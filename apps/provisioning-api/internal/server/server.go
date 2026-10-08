// Package server は Provisioning API の HTTPS サーバー（mTLS）を提供する。
package server

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/auth"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/handler"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
)

// NewEngine はミドルウェアとルーティングを設定した Gin のエンジンを返す。
func NewEngine(h *handler.Handler, clients auth.Clients, log *slog.Logger, masker *logging.Masker) *gin.Engine {
	engine := gin.New()
	engine.HandleMethodNotAllowed = true

	engine.Use(TraceIDMiddleware())
	engine.Use(LoggingMiddleware(log, masker))
	engine.Use(RecoveryMiddleware(log))
	engine.Use(MgmtClientMiddleware(clients))
	engine.Use(OperatorMiddleware())

	engine.NoRoute(func(c *gin.Context) {
		handler.WriteProblem(c, dto.NewProblem(http.StatusNotFound, "", "no such resource"))
	})
	engine.NoMethod(func(c *gin.Context) {
		handler.WriteProblem(c, dto.NewProblem(http.StatusMethodNotAllowed, "", "method not allowed for this resource"))
	})

	v1 := engine.Group("/admin/v1")
	v1.GET("/status", h.GetStatus)

	v1.GET("/subscribers", h.ListSubscribers)
	v1.POST("/subscribers", h.CreateSubscriber)
	v1.GET("/subscribers/:imsi", h.GetSubscriber)
	v1.PATCH("/subscribers/:imsi", h.UpdateSubscriber)
	v1.DELETE("/subscribers/:imsi", h.DeleteSubscriber)
	v1.GET("/subscribers/:imsi/keys", h.GetSubscriberKeys)

	v1.GET("/clients", h.ListClients)
	v1.POST("/clients", h.CreateClient)
	v1.GET("/clients/:clientId", h.GetClient)
	v1.PATCH("/clients/:clientId", h.UpdateClient)
	v1.DELETE("/clients/:clientId", h.DeleteClient)
	v1.GET("/clients/:clientId/secret", h.GetClientSecret)

	v1.GET("/policies", h.ListPolicies)
	v1.GET("/policies/:imsi", h.GetPolicy)
	v1.PUT("/policies/:imsi", h.PutPolicy)
	v1.DELETE("/policies/:imsi", h.DeletePolicy)

	return engine
}

// Server は HTTPS サーバーを管理する。
type Server struct {
	server *http.Server
	log    *slog.Logger
}

// New は新しい Server を生成する。tlsConfig には auth.Verifier.TLSConfig の設定を渡す。
func New(addr string, engine http.Handler, tlsConfig *tls.Config, log *slog.Logger) *Server {
	return &Server{
		server: &http.Server{
			Addr:              addr,
			Handler:           engine,
			TLSConfig:         tlsConfig,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      30 * time.Second,
			IdleTimeout:       120 * time.Second,
			// TLS ハンドシェイクの失敗等は DEBUG で残す（拒否した証明書は auth.Verifier が WARN で残す）
			ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelDebug),
		},
		log: log,
	}
}

// Run は待ち受けを始め、停止するまで戻らない。
func (s *Server) Run() error {
	ln, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}
	return s.Serve(ln)
}

// Serve は ln で TLS の待ち受けを行う（証明書は TLSConfig に設定済みのものを使う）。
func (s *Server) Serve(ln net.Listener) error {
	s.log.Info("starting server", "addr", ln.Addr().String())
	return s.server.ServeTLS(ln, "", "")
}

// Shutdown はサーバーを停止する。
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
