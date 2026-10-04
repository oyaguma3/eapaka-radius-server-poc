package server

import (
	"context"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
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
			// パケットの認証（Request Authenticator / Message-Authenticator）はハンドラーで検証し、
			// 失敗したら送信元IP付きのログを出して破棄する。ライブラリにも検証させると、
			// Accounting-Request などのシークレット不一致や未知のCodeのパケットが、
			// ハンドラーに届く前に素のテキストのログだけで捨てられてしまうため、ライブラリの検証は使わない
			InsecureSkipVerify: true,
			// ライブラリが出すエラー（パケットの解析失敗など）を JSON の slog に流す
			ErrorLog: logging.NewRADIUSLibraryLogger(),
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
