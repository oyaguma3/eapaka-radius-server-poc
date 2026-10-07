package server

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/auth"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/handler"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/service"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
	"github.com/redis/go-redis/v9"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newKeyPair(t *testing.T, cn string) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}
}

func TestMaskPath(t *testing.T) {
	tests := []struct {
		path   string
		masked bool
		want   string
	}{
		{"/admin/v1/subscribers/001010000000001/keys", true, "/admin/v1/subscribers/001010********1/keys"},
		{"/admin/v1/policies/00101000000001", true, "/admin/v1/policies/001010*******1"},
		{"/admin/v1/subscribers/001010000000001", false, "/admin/v1/subscribers/001010000000001"},
		{"/admin/v1/clients/192.168.10.1/secret", true, "/admin/v1/clients/192.168.10.1/secret"},
		{"/admin/v1/subscribers/1234567", true, "/admin/v1/subscribers/1234567"},
	}
	for _, tt := range tests {
		if got := MaskPath(tt.path, logging.NewMasker(tt.masked)); got != tt.want {
			t.Errorf("MaskPath(%q, %v) = %q, want %q", tt.path, tt.masked, got, tt.want)
		}
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	engine := gin.New()
	engine.Use(TraceIDMiddleware(), LoggingMiddleware(log, logging.NewMasker(true)), RecoveryMiddleware(log))
	engine.GET("/panic", func(*gin.Context) { panic("boom") })

	w := httptest.NewRecorder()
	engine.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError || !strings.Contains(w.Body.String(), "SYSTEM_FAILURE") {
		t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(buf.String(), "panic recovered") || !strings.Contains(buf.String(), `"http_status":500`) {
		t.Errorf("log = %s", buf.String())
	}
}

// TestServe は mTLS で待ち受け、登録済みのクライアントだけが API を使えることを確認する。
func TestServe(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	var logBuf, auditBuf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&logBuf, nil))
	serverCert := newKeyPair(t, "provisioning-api")
	clientCert := newKeyPair(t, "bff-01")
	clients := auth.Clients{auth.Fingerprint(clientCert.Leaf): "bff-01"}

	h := handler.New(service.New(rdb, audit.NewLogger(&auditBuf)), log, "0.1.0", "node-a", time.Now())
	verifier := &auth.Verifier{Clients: clients, Log: log}
	srv := New("127.0.0.1:0", NewEngine(h, clients, log, logging.NewMasker(true)), verifier.TLSConfig(serverCert), log)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()

	roots := x509.NewCertPool()
	roots.AddCert(serverCert.Leaf)
	client := func(certs ...tls.Certificate) *http.Client {
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, Certificates: certs}}}
	}
	base := "https://" + ln.Addr().String() + "/admin/v1"

	req, _ := http.NewRequest(http.MethodPost, base+"/clients", strings.NewReader(`{"ip":"192.168.10.1","secret":"s3cret","name":"AP-01"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Operator-Id", "alice")
	resp, err := client(clientCert).Do(req)
	if err != nil {
		t.Fatalf("POST /clients: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("status = %d", resp.StatusCode)
	}
	var a map[string]any
	if err := json.Unmarshal(auditBuf.Bytes(), &a); err != nil || a["mgmt_client"] != "bff-01" || a["admin_user"] != "alice" {
		t.Errorf("audit = %s", auditBuf.String())
	}

	if _, err := client(newKeyPair(t, "other")).Get(base + "/status"); err == nil {
		t.Error("unregistered client connected")
	}
	if !strings.Contains(logBuf.String(), "admin client certificate rejected") {
		t.Errorf("log = %s", logBuf.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
	if err := <-done; !errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Serve() = %v, want ErrServerClosed", err)
	}
}

func TestRun_ListenError(t *testing.T) {
	log := slog.New(slog.NewJSONHandler(io.Discard, nil))
	srv := New("256.0.0.1:0", http.NotFoundHandler(), &tls.Config{}, log)
	if err := srv.Run(); err == nil {
		t.Error("Run() expected error")
	}
}
