package server

import (
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
)

func TestNew_Timeouts(t *testing.T) {
	// 遅いクライアントや切れない接続でゴルーチンが溜まらないよう、HTTP サーバーに時間の上限がある。
	s := New(&config.Config{ListenAddr: "127.0.0.1:0", GinMode: gin.TestMode}, nil)
	if s.server.Addr != "127.0.0.1:0" {
		t.Errorf("Addr = %q", s.server.Addr)
	}
	if s.server.ReadHeaderTimeout != 10*time.Second || s.server.ReadTimeout != 30*time.Second ||
		s.server.WriteTimeout != 30*time.Second || s.server.IdleTimeout != 120*time.Second {
		t.Errorf("timeouts = %v %v %v %v", s.server.ReadHeaderTimeout, s.server.ReadTimeout, s.server.WriteTimeout, s.server.IdleTimeout)
	}
}
