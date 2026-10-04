package backend

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
	"encoding/pem"
	"errors"
	"io"
	"log"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
)

const testIMSI = "440101234567890"

// testVector はテスト用のEAP-AKAベクター。
var testVector = hssAuthVector{
	AVType: "EAP_AKA",
	RAND:   "0102030405060708090a0b0c0d0e0f10",
	XRES:   "2122232425262728",
	AUTN:   "1112131415161718191a1b1c1d1e1f20",
	CK:     "3132333435363738393a3b3c3d3e3f40",
	IK:     "4142434445464748494a4b4c4d4e4f50",
}

// testCert はテスト用に生成した証明書と秘密鍵。
type testCert struct {
	certPEM []byte
	keyPEM  []byte
	der     []byte
	tls     tls.Certificate
}

// genCert はaka-only-serverと同じ形式（ECDSA P-256の自己署名）の証明書を生成する。
// hostsが空ならクライアント認証用、そうでなければサーバー認証用とする。
func genCert(t *testing.T, cn string, hosts ...string) *testCert {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}
	if len(hosts) == 0 {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	} else {
		tmpl.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		for _, h := range hosts {
			if ip := net.ParseIP(h); ip != nil {
				tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
			} else {
				tmpl.DNSNames = append(tmpl.DNSNames, h)
			}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	c := &testCert{
		certPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		keyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
		der:     der,
	}
	c.tls, err = tls.X509KeyPair(c.certPEM, c.keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// writeFile はテンポラリディレクトリにファイルを書き、そのパスを返す。
func writeFile(t *testing.T, name string, data ...[]byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, bytes.Join(data, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// mtlsEnv はmTLSのテスト環境（サーバーと、それに接続するバックエンド）。
type mtlsEnv struct {
	srv     *httptest.Server
	client  *testCert
	backend *AKAOnlyBackend
}

// newMTLSEnv はクライアント証明書を要求するTLSサーバーを起動し、
// 証明書（クライアント証明書と秘密鍵は1ファイル）を設定したバックエンドを生成する。
func newMTLSEnv(t *testing.T, h http.HandlerFunc) *mtlsEnv {
	t.Helper()
	serverCert := genCert(t, "aka-only-server", "localhost", "127.0.0.1", "aka-only-server")
	clientCert := genCert(t, "radius-gw")

	srv := httptest.NewUnstartedServer(h)
	srv.TLS = &tls.Config{
		Certificates: []tls.Certificate{serverCert.tls},
		ClientAuth:   tls.RequireAnyClientCert,
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	b, err := NewAKAOnlyBackend(AKAOnlyOptions{
		BaseURL:        srv.URL,
		ClientCertFile: writeFile(t, "av-client.pem", clientCert.certPEM, clientCert.keyPEM),
		ServerCertFile: writeFile(t, "av-server.pem", serverCert.certPEM),
		Timeout:        5 * time.Second,
		MaskIMSI:       true,
	})
	if err != nil {
		t.Fatalf("NewAKAOnlyBackend() error = %v", err)
	}
	return &mtlsEnv{srv: srv, client: clientCert, backend: b}
}

// newPlainBackend は平文HTTPサーバーを起動し、それに接続するバックエンドを生成する。
func newPlainBackend(t *testing.T, h http.HandlerFunc, timeout time.Duration) *AKAOnlyBackend {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	b, err := NewAKAOnlyBackend(AKAOnlyOptions{BaseURL: srv.URL + "/", Timeout: timeout, MaskIMSI: true})
	if err != nil {
		t.Fatalf("NewAKAOnlyBackend() error = %v", err)
	}
	return b
}

// writeVector は成功レスポンスを書き込むハンドラー。
func writeVector(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(generateAVResponse{HSSAuthenticationVectors: []hssAuthVector{testVector}})
}

// writeProblem はaka-only-server形式のエラーを書き込むハンドラーを返す。
func writeProblem(status int, cause, detail string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(akaOnlyProblem{
			Title:  http.StatusText(status),
			Status: status,
			Detail: detail,
			Cause:  cause,
		})
	}
}

// captureLog はテスト中のslog出力をバッファに取り込む。
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func assertVector(t *testing.T, got *VectorResponse) {
	t.Helper()
	want := VectorResponse{RAND: testVector.RAND, AUTN: testVector.AUTN, XRES: testVector.XRES, CK: testVector.CK, IK: testVector.IK}
	if *got != want {
		t.Errorf("GetVector() = %+v, want %+v", *got, want)
	}
}

func TestAKAOnlyBackend_GetVector_MTLS(t *testing.T) {
	var (
		gotMethod, gotPath, gotTraceID, gotContentType string
		gotBody                                        map[string]any
		gotPeer                                        []byte
	)
	env := newMTLSEnv(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotTraceID = r.Header.Get("X-Trace-ID")
		gotContentType = r.Header.Get("Content-Type")
		json.NewDecoder(r.Body).Decode(&gotBody)
		if len(r.TLS.PeerCertificates) > 0 {
			gotPeer = r.TLS.PeerCertificates[0].Raw
		}
		writeVector(w, r)
	})
	logBuf := captureLog(t)

	ctx := ContextWithTraceID(context.Background(), "trace-mtls")
	resp, err := env.backend.GetVector(ctx, &VectorRequest{IMSI: testIMSI})
	if err != nil {
		t.Fatalf("GetVector() error = %v", err)
	}
	assertVector(t, resp)

	if gotMethod != http.MethodPost {
		t.Errorf("Method = %q, want POST", gotMethod)
	}
	wantPath := "/nudm-ueau/v1/imsi-" + testIMSI + "/hss-security-information/eap-aka/generate-av"
	if gotPath != wantPath {
		t.Errorf("Path = %q, want %q", gotPath, wantPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", gotContentType)
	}
	if gotTraceID != "trace-mtls" {
		t.Errorf("X-Trace-ID = %q, want %q", gotTraceID, "trace-mtls")
	}
	if gotBody["hssAuthType"] != "EAP_AKA" {
		t.Errorf("hssAuthType = %v, want EAP_AKA", gotBody["hssAuthType"])
	}
	if gotBody["numOfRequestedVectors"] != float64(1) {
		t.Errorf("numOfRequestedVectors = %v, want 1", gotBody["numOfRequestedVectors"])
	}
	for _, k := range []string{"resynchronizationInfo", "anId"} {
		if _, ok := gotBody[k]; ok {
			t.Errorf("%s must not be sent: %v", k, gotBody)
		}
	}
	if !bytes.Equal(gotPeer, env.client.der) {
		t.Error("server did not receive the configured client certificate")
	}

	// ログ: BACKEND_EXTERNAL_CALL、IMSIはマスク済み、鍵素材は出さない
	logs := logBuf.String()
	if !strings.Contains(logs, `"event_id":"BACKEND_EXTERNAL_CALL"`) {
		t.Errorf("BACKEND_EXTERNAL_CALL not logged: %s", logs)
	}
	if !strings.Contains(logs, `"backend_id":"01"`) || !strings.Contains(logs, `"trace_id":"trace-mtls"`) {
		t.Errorf("backend_id/trace_id not logged: %s", logs)
	}
	for _, secret := range []string{testIMSI, testVector.CK, testVector.IK, testVector.XRES} {
		if strings.Contains(logs, secret) {
			t.Errorf("log contains sensitive value %q: %s", secret, logs)
		}
	}
	if !env.backend.UseTLS() {
		t.Error("UseTLS() = false, want true")
	}
}

func TestAKAOnlyBackend_GetVector_SeparateKeyFile(t *testing.T) {
	serverCert := genCert(t, "aka-only-server", "127.0.0.1")
	clientCert := genCert(t, "radius-gw")
	srv := httptest.NewUnstartedServer(http.HandlerFunc(writeVector))
	srv.TLS = &tls.Config{Certificates: []tls.Certificate{serverCert.tls}, ClientAuth: tls.RequireAnyClientCert}
	srv.StartTLS()
	defer srv.Close()

	b, err := NewAKAOnlyBackend(AKAOnlyOptions{
		BaseURL:        srv.URL,
		ClientCertFile: writeFile(t, "client.crt", clientCert.certPEM),
		ClientKeyFile:  writeFile(t, "client.key", clientCert.keyPEM),
		ServerCertFile: writeFile(t, "server.crt", serverCert.certPEM),
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewAKAOnlyBackend() error = %v", err)
	}
	resp, err := b.GetVector(context.Background(), &VectorRequest{IMSI: testIMSI})
	if err != nil {
		t.Fatalf("GetVector() error = %v", err)
	}
	assertVector(t, resp)
}

func TestAKAOnlyBackend_GetVector_Plain(t *testing.T) {
	var gotPath string
	b := newPlainBackend(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		writeVector(w, r)
	}, 5*time.Second)

	resp, err := b.GetVector(context.Background(), &VectorRequest{IMSI: testIMSI})
	if err != nil {
		t.Fatalf("GetVector() error = %v", err)
	}
	assertVector(t, resp)
	// ベースURL末尾の"/"は取り除かれる
	if !strings.HasPrefix(gotPath, "/nudm-ueau/") {
		t.Errorf("Path = %q, want prefix /nudm-ueau/", gotPath)
	}
	if b.UseTLS() {
		t.Error("UseTLS() = true, want false")
	}
}

func TestAKAOnlyBackend_GetVector_Resync(t *testing.T) {
	var gotBody generateAVRequest
	b := newPlainBackend(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		writeVector(w, r)
	}, 5*time.Second)

	req := &VectorRequest{
		IMSI:       testIMSI,
		ResyncInfo: &ResyncInfo{RAND: "aa112233445566778899aabbccddeeff", AUTS: "0102030405060708090a0b0c0d0e"},
	}
	if _, err := b.GetVector(context.Background(), req); err != nil {
		t.Fatalf("GetVector() error = %v", err)
	}
	if gotBody.ResynchronizationInfo == nil {
		t.Fatal("resynchronizationInfo not sent")
	}
	if gotBody.ResynchronizationInfo.RAND != req.ResyncInfo.RAND || gotBody.ResynchronizationInfo.AUTS != req.ResyncInfo.AUTS {
		t.Errorf("resynchronizationInfo = %+v, want %+v", *gotBody.ResynchronizationInfo, *req.ResyncInfo)
	}
	if gotBody.HSSAuthType != "EAP_AKA" || gotBody.NumOfRequestedVectors != 1 {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestAKAOnlyBackend_GetVector_UntrustedServer(t *testing.T) {
	sameCert := genCert(t, "aka-only-server", "aka-only-server")
	tests := []struct {
		name string
		// configuredはバックエンドに設定するサーバー証明書、actualはサーバーが使う証明書
		configured, actual *testCert
	}{
		{"different certificate", genCert(t, "trusted", "127.0.0.1"), genCert(t, "untrusted", "127.0.0.1")},
		// 信頼した証明書でも、URLのホスト名（127.0.0.1）がSANになければ接続しない
		{"hostname not in SAN", sameCert, sameCert},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				writeVector(w, r)
			}))
			srv.TLS = &tls.Config{Certificates: []tls.Certificate{tt.actual.tls}, ClientAuth: tls.RequireAnyClientCert}
			srv.Config.ErrorLog = log.New(io.Discard, "", 0)
			srv.StartTLS()
			defer srv.Close()

			clientCert := genCert(t, "radius-gw")
			b, err := NewAKAOnlyBackend(AKAOnlyOptions{
				BaseURL:        srv.URL,
				ClientCertFile: writeFile(t, "client.pem", clientCert.certPEM, clientCert.keyPEM),
				ServerCertFile: writeFile(t, "server.pem", tt.configured.certPEM),
				Timeout:        5 * time.Second,
			})
			if err != nil {
				t.Fatalf("NewAKAOnlyBackend() error = %v", err)
			}

			_, err = b.GetVector(context.Background(), &VectorRequest{IMSI: testIMSI})
			var commErr *BackendCommunicationError
			if !errors.As(err, &commErr) {
				t.Fatalf("expected BackendCommunicationError, got %v", err)
			}
			if called {
				t.Error("request reached the untrusted server")
			}
		})
	}
}

func TestAKAOnlyBackend_GetVector_ResponseErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		cause  string
		detail string
	}{
		{"user not found", http.StatusNotFound, "USER_NOT_FOUND", "subscriber not found"},
		{"client not allowed", http.StatusForbidden, "AUTHENTICATION_REJECTED", "client is not allowed for this subscriber"},
		{"auts verification failed", http.StatusForbidden, "AUTHENTICATION_REJECTED", "AUTS verification failed"},
		{"invalid format", http.StatusBadRequest, "INVALID_MSG_FORMAT", "request body is not valid JSON for this operation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newMTLSEnv(t, writeProblem(tt.status, tt.cause, tt.detail))
			logBuf := captureLog(t)

			_, err := env.backend.GetVector(context.Background(), &VectorRequest{IMSI: testIMSI})
			var respErr *BackendResponseError
			if !errors.As(err, &respErr) {
				t.Fatalf("expected BackendResponseError, got %v", err)
			}
			if respErr.StatusCode != tt.status {
				t.Errorf("StatusCode = %d, want %d", respErr.StatusCode, tt.status)
			}
			p := respErr.Problem
			if p.Type != "about:blank" || p.Status != tt.status || p.Title != http.StatusText(tt.status) || p.Detail != tt.detail {
				t.Errorf("Problem = %+v", *p)
			}

			logs := logBuf.String()
			if !strings.Contains(logs, `"event_id":"BACKEND_EXTERNAL_ERR"`) || !strings.Contains(logs, `"cause":"`+tt.cause+`"`) {
				t.Errorf("BACKEND_EXTERNAL_ERR with cause not logged: %s", logs)
			}
			if !strings.Contains(logs, `"level":"WARN"`) {
				t.Errorf("4xx should be logged at WARN: %s", logs)
			}
		})
	}
}

func TestAKAOnlyBackend_GetVector_TitleFallback(t *testing.T) {
	b := newPlainBackend(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		io.WriteString(w, `{"status":403,"cause":"AUTHENTICATION_REJECTED"}`)
	}, 5*time.Second)

	_, err := b.GetVector(context.Background(), &VectorRequest{IMSI: testIMSI})
	var respErr *BackendResponseError
	if !errors.As(err, &respErr) {
		t.Fatalf("expected BackendResponseError, got %v", err)
	}
	if respErr.Problem.Title != "Forbidden" {
		t.Errorf("Title = %q, want Forbidden", respErr.Problem.Title)
	}
}

func TestAKAOnlyBackend_GetVector_CommunicationErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{"system failure", writeProblem(http.StatusInternalServerError, "SYSTEM_FAILURE", "")},
		{"auth type not supported", writeProblem(http.StatusNotImplemented, "AUTH_TYPE_NOT_SUPPORTED", "auth type eap-aka is not supported")},
		{"503 without body", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }},
		{"4xx not from aka-only-server", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) }},
		{"4xx broken json", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"cause":`)
		}},
		{"unexpected status", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }},
		{"empty vectors", func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, `{"hssAuthenticationVectors":[]}`)
		}},
		{"no vectors field", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, `{}`) }},
		{"broken json", func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, `{"hssAuthenticationVectors":[`) }},
		{"missing ck", func(w http.ResponseWriter, _ *http.Request) {
			v := testVector
			v.CK = ""
			json.NewEncoder(w).Encode(generateAVResponse{HSSAuthenticationVectors: []hssAuthVector{v}})
		}},
		{"eap-aka-prime vector", func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, `{"hssAuthenticationVectors":[{"avType":"EAP_AKA_PRIME","rand":"01","xres":"02","autn":"03","ckPrime":"04","ikPrime":"05"}]}`)
		}},
		{"oversized body", func(w http.ResponseWriter, _ *http.Request) {
			io.WriteString(w, `{"pad":"`+strings.Repeat("a", akaOnlyMaxResponseBytes)+`"}`)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newMTLSEnv(t, tt.handler)
			logBuf := captureLog(t)

			_, err := env.backend.GetVector(context.Background(), &VectorRequest{IMSI: testIMSI})
			var commErr *BackendCommunicationError
			if !errors.As(err, &commErr) {
				t.Fatalf("expected BackendCommunicationError, got %v", err)
			}
			logs := logBuf.String()
			if !strings.Contains(logs, `"event_id":"BACKEND_EXTERNAL_ERR"`) || !strings.Contains(logs, `"level":"ERROR"`) {
				t.Errorf("BACKEND_EXTERNAL_ERR at ERROR not logged: %s", logs)
			}
		})
	}
}

func TestAKAOnlyBackend_GetVector_5xxCauseLogged(t *testing.T) {
	env := newMTLSEnv(t, writeProblem(http.StatusNotImplemented, "AUTH_TYPE_NOT_SUPPORTED", ""))
	logBuf := captureLog(t)

	_, _ = env.backend.GetVector(context.Background(), &VectorRequest{IMSI: testIMSI})
	logs := logBuf.String()
	if !strings.Contains(logs, `"cause":"AUTH_TYPE_NOT_SUPPORTED"`) || !strings.Contains(logs, `"http_status":501`) {
		t.Errorf("cause/http_status not logged: %s", logs)
	}
}

func TestAKAOnlyBackend_GetVector_Timeout(t *testing.T) {
	done := make(chan struct{})
	b := newPlainBackend(t, func(w http.ResponseWriter, r *http.Request) {
		// ボディを読み切らないとサーバーがクライアントの切断を検知しない
		io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}, 100*time.Millisecond)
	// サーバーのClose（先に登録したCleanup）より前にハンドラーを解放する
	t.Cleanup(func() { close(done) })

	_, err := b.GetVector(context.Background(), &VectorRequest{IMSI: testIMSI})
	var commErr *BackendCommunicationError
	if !errors.As(err, &commErr) {
		t.Fatalf("expected BackendCommunicationError, got %v", err)
	}
}

func TestAKAOnlyBackend_GetVector_ConnectionErrorHidesIMSI(t *testing.T) {
	b, err := NewAKAOnlyBackend(AKAOnlyOptions{BaseURL: "http://127.0.0.1:1", Timeout: time.Second, MaskIMSI: true})
	if err != nil {
		t.Fatal(err)
	}
	logBuf := captureLog(t)

	_, err = b.GetVector(context.Background(), &VectorRequest{IMSI: testIMSI})
	var commErr *BackendCommunicationError
	if !errors.As(err, &commErr) {
		t.Fatalf("expected BackendCommunicationError, got %v", err)
	}
	// エラーメッセージとログにIMSI（URLのパス）を含めない
	if strings.Contains(err.Error(), testIMSI) {
		t.Errorf("error message contains IMSI: %v", err)
	}
	if strings.Contains(logBuf.String(), testIMSI) {
		t.Errorf("log contains IMSI: %s", logBuf.String())
	}
}

func TestNewAKAOnlyBackend_Errors(t *testing.T) {
	serverCert := genCert(t, "aka-only-server", "127.0.0.1")
	clientCert := genCert(t, "radius-gw")
	otherClient := genCert(t, "other")
	serverPath := writeFile(t, "server.pem", serverCert.certPEM)
	clientPath := writeFile(t, "client.pem", clientCert.certPEM, clientCert.keyPEM)
	certOnlyPath := writeFile(t, "client.crt", clientCert.certPEM)
	garbagePath := writeFile(t, "garbage.pem", []byte("not a pem"))
	missingPath := filepath.Join(t.TempDir(), "missing.pem")

	tests := []struct {
		name string
		opts AKAOnlyOptions
	}{
		{"unsupported scheme", AKAOnlyOptions{BaseURL: "ftp://aka-only-server"}},
		{"https without client cert", AKAOnlyOptions{BaseURL: "https://127.0.0.1:8443", ServerCertFile: serverPath}},
		{"https without server cert", AKAOnlyOptions{BaseURL: "https://127.0.0.1:8443", ClientCertFile: clientPath}},
		{"client cert not found", AKAOnlyOptions{BaseURL: "https://127.0.0.1:8443", ClientCertFile: missingPath, ServerCertFile: serverPath}},
		{"client key not found", AKAOnlyOptions{BaseURL: "https://127.0.0.1:8443", ClientCertFile: certOnlyPath, ClientKeyFile: missingPath, ServerCertFile: serverPath}},
		{"client cert without key", AKAOnlyOptions{BaseURL: "https://127.0.0.1:8443", ClientCertFile: certOnlyPath, ServerCertFile: serverPath}},
		{"client key mismatch", AKAOnlyOptions{
			BaseURL:        "https://127.0.0.1:8443",
			ClientCertFile: certOnlyPath,
			ClientKeyFile:  writeFile(t, "other.key", otherClient.keyPEM),
			ServerCertFile: serverPath,
		}},
		{"server cert not found", AKAOnlyOptions{BaseURL: "https://127.0.0.1:8443", ClientCertFile: clientPath, ServerCertFile: missingPath}},
		{"server cert garbage", AKAOnlyOptions{BaseURL: "https://127.0.0.1:8443", ClientCertFile: clientPath, ServerCertFile: garbagePath}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewAKAOnlyBackend(tt.opts); err == nil {
				t.Error("NewAKAOnlyBackend() expected error")
			}
		})
	}
}

func TestAKAOnlyBackend_IDAndName(t *testing.T) {
	b, err := NewAKAOnlyBackend(AKAOnlyOptions{BaseURL: "http://aka-only-server:8080", Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if b.ID() != "01" {
		t.Errorf("ID() = %q, want %q", b.ID(), "01")
	}
	if b.Name() != "aka-only-server" {
		t.Errorf("Name() = %q, want %q", b.Name(), "aka-only-server")
	}
}

func TestNewRegistry_AKAOnly(t *testing.T) {
	serverCert := genCert(t, "aka-only-server", "127.0.0.1")
	clientCert := genCert(t, "radius-gw")

	t.Run("registered when URL is set", func(t *testing.T) {
		cfg := &config.Config{
			InternalURL:       "http://localhost:8080",
			InternalTimeout:   5 * time.Second,
			AKAOnlyURL:        "https://aka-only-server:8443",
			AKAOnlyClientCert: writeFile(t, "client.pem", clientCert.certPEM, clientCert.keyPEM),
			AKAOnlyServerCert: writeFile(t, "server.pem", serverCert.certPEM),
			AKAOnlyTimeout:    5 * time.Second,
		}
		r, err := NewRegistry(cfg)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		b, err := r.Get("01")
		if err != nil {
			t.Fatalf("Get(\"01\") error = %v", err)
		}
		if b.ID() != "01" {
			t.Errorf("ID() = %q, want %q", b.ID(), "01")
		}
		// デフォルトは00のまま
		if r.Default().ID() != "00" {
			t.Errorf("Default().ID() = %q, want %q", r.Default().ID(), "00")
		}
	})

	t.Run("not registered when URL is empty", func(t *testing.T) {
		cfg := &config.Config{InternalURL: "http://localhost:8080", InternalTimeout: 5 * time.Second}
		r, err := NewRegistry(cfg)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		_, err = r.Get("01")
		var notImpl *BackendNotImplementedError
		if !errors.As(err, &notImpl) {
			t.Fatalf("expected BackendNotImplementedError, got %v", err)
		}
	})

	t.Run("error when certificates are invalid", func(t *testing.T) {
		cfg := &config.Config{
			InternalURL:       "http://localhost:8080",
			InternalTimeout:   5 * time.Second,
			AKAOnlyURL:        "https://aka-only-server:8443",
			AKAOnlyClientCert: filepath.Join(t.TempDir(), "missing.pem"),
			AKAOnlyServerCert: writeFile(t, "server.pem", serverCert.certPEM),
			AKAOnlyTimeout:    5 * time.Second,
		}
		if _, err := NewRegistry(cfg); err == nil {
			t.Error("NewRegistry() expected error")
		}
	})
}
