package server

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
	radiuspkg "layeh.com/radius"
)

// syncBuffer は複数のゴルーチンから書き込まれるログを集める。
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// waitFor は ログに want が現れるまで待つ。
func (b *syncBuffer) waitFor(t *testing.T, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(b.String(), want) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("log does not contain %s: %s", want, b.String())
}

// startTestServer は NewServer と同じ設定の PacketServer をループバックの空きポートで起動し、
// クライアント用の UDP 接続を返す。
func startTestServer(t *testing.T, proc *mockProcessor, secret []byte) (net.Conn, *syncBuffer) {
	t.Helper()

	logs := &syncBuffer{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("ListenPacket failed: %v", err)
	}

	s := NewServer(pc.LocalAddr().String(), NewHandler(proc), radiuspkg.StaticSecretSource(secret))
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

	return conn, logs
}

// encodeAccountingStart は secret で Request Authenticator を計算した Accounting-Request（Start）を返す。
func encodeAccountingStart(t *testing.T, secret []byte) []byte {
	t.Helper()
	b, err := createAccountingRequest(t, secret, radius.AcctStatusTypeStart).Packet.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}
	return b
}

// readResponse は応答を待ち、届かなければ nil を返す。
func readResponse(t *testing.T, conn net.Conn, wait time.Duration) []byte {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(wait))
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return nil
	}
	return buf[:n]
}

func TestNewServer_PacketServerSettings(t *testing.T) {
	s := NewServer(":1813", NewHandler(&mockProcessor{}), radiuspkg.StaticSecretSource([]byte("secret")))

	if s.ps.Addr != ":1813" {
		t.Errorf("Addr = %q, want :1813", s.ps.Addr)
	}
	if !s.ps.InsecureSkipVerify {
		t.Error("InsecureSkipVerify must be true (packets are verified by the handler)")
	}
	if s.ps.ErrorLog == nil {
		t.Error("ErrorLog must be set")
	}
}

func TestServer_AccountingRequest_Valid(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	conn, _ := startTestServer(t, proc, secret)

	if _, err := conn.Write(encodeAccountingStart(t, secret)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	resp := readResponse(t, conn, 2*time.Second)
	if resp == nil {
		t.Fatal("expected Accounting-Response")
	}
	if radiuspkg.Code(resp[0]) != radiuspkg.CodeAccountingResponse {
		t.Errorf("response code = %d, want Accounting-Response", resp[0])
	}
}

func TestServer_AccountingRequest_WrongSecret(t *testing.T) {
	proc := &mockProcessor{}
	conn, logs := startTestServer(t, proc, []byte("testing123"))

	// NAS 側のシークレットが違う（ライブラリでは捨てず、ハンドラーが検証して捨てる）
	if _, err := conn.Write(encodeAccountingStart(t, []byte("wrong-secret"))); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	logs.waitFor(t, `"event_id":"RADIUS_AUTH_ERR"`)
	if !strings.Contains(logs.String(), `"src_ip":"127.0.0.1"`) {
		t.Errorf("RADIUS_AUTH_ERR should have src_ip: %s", logs.String())
	}
	if readResponse(t, conn, 200*time.Millisecond) != nil {
		t.Error("no response is expected for a wrong secret")
	}
	if proc.startCalled {
		t.Error("ProcessStart must not be called for a wrong secret")
	}
	// ライブラリの素のテキストのログ（bad secret）は出ない
	if strings.Contains(logs.String(), "bad secret") {
		t.Errorf("library validation log should not appear: %s", logs.String())
	}
}

func TestServer_UnknownCode(t *testing.T) {
	conn, logs := startTestServer(t, &mockProcessor{}, []byte("testing123"))

	// Accounting で扱わない Code（Access-Accept）も、ハンドラーに届いて記録される
	p := radiuspkg.New(radiuspkg.CodeAccessAccept, []byte("testing123"))
	b, err := p.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}
	if _, err := conn.Write(b); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	logs.waitFor(t, `"event_id":"RADIUS_UNKNOWN_CODE"`)
}

func TestServer_MalformedPacket(t *testing.T) {
	conn, logs := startTestServer(t, &mockProcessor{}, []byte("testing123"))

	// 20バイトに満たないパケットは、ライブラリの解析エラーとして JSON のログになる
	if _, err := conn.Write([]byte{4, 1, 0, 10, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	logs.waitFor(t, `"event_id":"RADIUS_LIB_ERR"`)
	if !strings.Contains(logs.String(), `"level":"WARN"`) {
		t.Errorf("RADIUS_LIB_ERR should be WARN: %s", logs.String())
	}
}
