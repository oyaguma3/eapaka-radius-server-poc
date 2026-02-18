package server

import (
	"context"

	"layeh.com/radius"
)

// Server はRADIUS UDPサーバーのラッパー
type Server struct {
	ps *radius.PacketServer
}

// NewServer は新しいServerを生成する
func NewServer(addr string, handler radius.Handler, secretSource radius.SecretSource) *Server {
	return &Server{
		ps: &radius.PacketServer{
			Addr:         addr,
			SecretSource: secretSource,
			Handler:      handler,
		},
	}
}

// ListenAndServe はUDPサーバーを起動する
func (s *Server) ListenAndServe() error {
	return s.ps.ListenAndServe()
}

// Shutdown はサーバーをグレースフルに停止する
func (s *Server) Shutdown(ctx context.Context) error {
	return s.ps.Shutdown(ctx)
}
