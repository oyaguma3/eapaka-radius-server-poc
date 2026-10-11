package server

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"

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
			Handler:      recoverHandler(handler),
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

// recoverHandler は、パケットの処理中のパニックを回復してログ（SYS_PANIC）に出す。
// layeh.com/radius の PacketServer はパケットごとの goroutine でハンドラーを呼び、パニックを回復しないので、
// 回復しないと 1 つのパケットでプロセスごと落ちる。そのパケットには応答しない（NAS は応答がなければ送り直す）。
func recoverHandler(next radius.Handler) radius.Handler {
	return radius.HandlerFunc(func(w radius.ResponseWriter, r *radius.Request) {
		defer func() {
			if p := recover(); p != nil {
				slog.Error("パケット処理中のパニック（回復して処理を続ける）",
					"event_id", "SYS_PANIC",
					"src_ip", extractIP(r.RemoteAddr),
					"code", r.Code,
					"panic", fmt.Sprint(p),
					"stack", string(debug.Stack()),
				)
			}
		}()
		next.ServeRADIUS(w, r)
	})
}

// ListenAndServe はUDPサーバーを起動する
func (s *Server) ListenAndServe() error {
	return s.ps.ListenAndServe()
}

// Shutdown はサーバーをグレースフルに停止する
func (s *Server) Shutdown(ctx context.Context) error {
	return s.ps.Shutdown(ctx)
}
