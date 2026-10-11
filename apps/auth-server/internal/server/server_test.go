package server

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"layeh.com/radius"
)

func TestNewServer(t *testing.T) {
	handler := radius.HandlerFunc(func(w radius.ResponseWriter, r *radius.Request) {})
	secretSource := radius.StaticSecretSource([]byte("test-secret"))

	s := NewServer(":1812", handler, secretSource)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.ps == nil {
		t.Fatal("PacketServer is nil")
	}
	if s.ps.Addr != ":1812" {
		t.Errorf("Addr: got %q, want %q", s.ps.Addr, ":1812")
	}
}

func TestNewServer_CustomAddr(t *testing.T) {
	handler := radius.HandlerFunc(func(w radius.ResponseWriter, r *radius.Request) {})
	secretSource := radius.StaticSecretSource([]byte("secret"))

	s := NewServer(":1813", handler, secretSource)
	if s.ps.Addr != ":1813" {
		t.Errorf("Addr: got %q, want %q", s.ps.Addr, ":1813")
	}
}

func TestNewServer_PacketServerSettings(t *testing.T) {
	handler := radius.HandlerFunc(func(w radius.ResponseWriter, r *radius.Request) {})
	s := NewServer(":1812", handler, radius.StaticSecretSource([]byte("secret")))

	if !s.ps.InsecureSkipVerify {
		t.Error("InsecureSkipVerify must be true (packets are verified by the handler)")
	}
	if s.ps.ErrorLog == nil {
		t.Error("ErrorLog must be set")
	}
}

// TestServer_PacketsReachHandler は、ライブラリで検証される種類のパケット（Accounting-Request）や
// 未知の Code のパケットも、ライブラリに捨てられずにハンドラーへ届くことを確かめる
// （ハンドラーが PKT_UNKNOWN_CODE を送信元IP付きで記録できるようにするため）。
func TestServer_PacketsReachHandler(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket failed: %v", err)
	}

	received := make(chan radius.Code, 4)
	handler := radius.HandlerFunc(func(w radius.ResponseWriter, r *radius.Request) {
		received <- r.Code
	})
	s := NewServer(pc.LocalAddr().String(), handler, radius.StaticSecretSource([]byte("testing123")))
	go func() { _ = s.ps.Serve(pc) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	})

	conn, err := net.Dial("udp", pc.LocalAddr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	for _, code := range []radius.Code{radius.CodeAccountingRequest, radius.Code(99)} {
		// 別のシークレットで Request Authenticator を計算した（ライブラリの検証なら捨てられる）パケット
		p := radius.New(code, []byte("wrong-secret"))
		b, err := p.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}
		if _, err := conn.Write(b); err != nil {
			t.Fatalf("Write failed: %v", err)
		}

		select {
		case got := <-received:
			if got != code {
				t.Errorf("handler received code %d, want %d", got, code)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("packet with code %d did not reach the handler", code)
		}
	}
}

func TestRecoverHandler(t *testing.T) {
	// パケットの処理中のパニックは回復し、SYS_PANIC のログを出す（プロセスは落ちない）。
	var buf bytes.Buffer
	orig := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(orig) })

	h := recoverHandler(radius.HandlerFunc(func(radius.ResponseWriter, *radius.Request) {
		panic("boom")
	}))
	req := &radius.Request{
		Packet:     radius.New(radius.CodeAccessRequest, []byte("secret")),
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 50000},
	}
	h.ServeRADIUS(nil, req)

	out := buf.String()
	for _, want := range []string{`"event_id":"SYS_PANIC"`, `"src_ip":"192.0.2.10"`, `"panic":"boom"`, `"stack":`} {
		if !strings.Contains(out, want) {
			t.Errorf("log does not contain %s: %s", want, out)
		}
	}
}

func TestRecoverHandler_PassThrough(t *testing.T) {
	called := false
	h := recoverHandler(radius.HandlerFunc(func(radius.ResponseWriter, *radius.Request) { called = true }))
	h.ServeRADIUS(nil, &radius.Request{Packet: radius.New(radius.CodeAccessRequest, []byte("secret"))})
	if !called {
		t.Error("handler was not called")
	}
}
