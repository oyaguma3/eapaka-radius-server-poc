package main

import (
	"bytes"
	"log/slog"
	"net"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/server"
	"layeh.com/radius"
)

func TestWarnFallbackSecret(t *testing.T) {
	tests := []struct {
		name     string
		secret   string
		wantWarn bool
	}{
		{"設定ありならWARNを出す", "testing123", true},
		{"空なら出さない", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			t.Cleanup(func() { slog.SetDefault(prev) })

			warnFallbackSecret(tt.secret)

			out := buf.String()
			if got := strings.Contains(out, `"level":"WARN"`); got != tt.wantWarn {
				t.Errorf("WARN logged = %v, want %v: %s", got, tt.wantWarn, out)
			}
			// シークレットの値はログに出さない
			if tt.secret != "" && strings.Contains(out, tt.secret) {
				t.Errorf("log must not contain the secret: %s", out)
			}
		})
	}
}

func TestRunServer_ListenFailure(t *testing.T) {
	// ポートが使用中で待ち受けを開始できなければ、シグナルを待たずにエラーを返す（main は終了コード 1 で終わる）。
	occupied, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer occupied.Close()
	addr := occupied.LocalAddr().String()
	srv := server.NewServer(addr, radius.HandlerFunc(func(radius.ResponseWriter, *radius.Request) {}),
		radius.StaticSecretSource([]byte("secret")))

	done := make(chan error, 1)
	go func() { done <- runServer(srv, addr, make(chan os.Signal), time.Second) }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("runServer returned nil, want listen error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runServer did not return although the port is in use")
	}
}

func TestRunServer_Signal(t *testing.T) {
	// シグナルを受けたらグレースフルに停止して nil を返す。
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := pc.LocalAddr().String()
	pc.Close()
	srv := server.NewServer(addr, radius.HandlerFunc(func(radius.ResponseWriter, *radius.Request) {}),
		radius.StaticSecretSource([]byte("secret")))

	sigCh := make(chan os.Signal, 1)
	done := make(chan error, 1)
	go func() { done <- runServer(srv, addr, sigCh, time.Second) }()
	time.Sleep(100 * time.Millisecond) // 待ち受けを始めるのを待つ
	sigCh <- syscall.SIGTERM
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runServer = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runServer did not return after the signal")
	}
}
