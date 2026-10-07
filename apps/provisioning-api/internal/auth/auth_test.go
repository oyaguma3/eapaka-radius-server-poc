package auth

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newKeyPair は自己署名の証明書と秘密鍵を生成する。
func newKeyPair(t *testing.T, cn string, notBefore, notAfter time.Time) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		DNSNames:     []string{"localhost"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
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

func validKeyPair(t *testing.T, cn string) tls.Certificate {
	return newKeyPair(t, cn, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
}

func TestParseClients(t *testing.T) {
	fpA := strings.Repeat("a", 64)
	fpB := strings.Repeat("B", 64)
	colon := strings.TrimSuffix(strings.Repeat("CC:", 32), ":")

	got, err := ParseClients(" bff-01=" + fpA + " , bff.02 = " + fpB + ",ops_3=" + colon + ",")
	if err != nil {
		t.Fatalf("ParseClients() error = %v", err)
	}
	want := Clients{fpA: "bff-01", strings.ToLower(fpB): "bff.02", strings.Repeat("c", 64): "ops_3"}
	if len(got) != len(want) {
		t.Fatalf("ParseClients() = %v, want %v", got, want)
	}
	for fp, name := range want {
		if got[fp] != name {
			t.Errorf("clients[%s] = %q, want %q", fp, got[fp], name)
		}
	}

	if got, err := ParseClients(""); err != nil || len(got) != 0 {
		t.Errorf("ParseClients(\"\") = %v, %v", got, err)
	}

	for _, bad := range []string{
		"bff-01",             // = がない
		"=" + fpA,            // 識別名がない
		"bad name=" + fpA,    // 識別名に空白
		"bff-01=" + fpA[:63], // 63桁
		"bff-01=" + strings.Repeat("g", 64),
		"bff-01=" + fpA + ",bff-02=" + fpA, // 重複
	} {
		if _, err := ParseClients(bad); err == nil {
			t.Errorf("ParseClients(%q) expected error", bad)
		}
	}
}

func TestVerifyConnection(t *testing.T) {
	registered := validKeyPair(t, "bff-01")
	var logBuf bytes.Buffer
	v := &Verifier{
		Clients: Clients{Fingerprint(registered.Leaf): "bff-01"},
		Log:     slog.New(slog.NewJSONHandler(&logBuf, nil)),
	}

	if err := v.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{registered.Leaf}}); err != nil {
		t.Errorf("registered certificate rejected: %v", err)
	}
	if err := v.VerifyConnection(tls.ConnectionState{}); err == nil {
		t.Error("no certificate accepted")
	}
	other := validKeyPair(t, "other")
	if err := v.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{other.Leaf}}); err == nil {
		t.Error("unregistered certificate accepted")
	}
	if !strings.Contains(logBuf.String(), Fingerprint(other.Leaf)) || !strings.Contains(logBuf.String(), "PROV_CLIENT_REJECTED") {
		t.Errorf("rejection log = %s", logBuf.String())
	}

	// 有効期間外
	for _, now := range []time.Time{registered.Leaf.NotBefore.Add(-time.Minute), registered.Leaf.NotAfter.Add(time.Minute)} {
		v.Now = func() time.Time { return now }
		if err := v.VerifyConnection(tls.ConnectionState{PeerCertificates: []*x509.Certificate{registered.Leaf}}); err == nil {
			t.Errorf("certificate accepted at %v", now)
		}
	}

	// ログがなくても動く
	v = &Verifier{Clients: Clients{}}
	if err := v.VerifyConnection(tls.ConnectionState{}); err == nil {
		t.Error("no certificate accepted")
	}
}

func TestNameFromRequest(t *testing.T) {
	kp := validKeyPair(t, "bff-01")
	clients := Clients{Fingerprint(kp.Leaf): "bff-01"}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := clients.NameFromRequest(r); got != "" {
		t.Errorf("NameFromRequest() without TLS = %q", got)
	}
	r.TLS = &tls.ConnectionState{}
	if got := clients.NameFromRequest(r); got != "" {
		t.Errorf("NameFromRequest() without certificate = %q", got)
	}
	r.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{kp.Leaf}}
	if got := clients.NameFromRequest(r); got != "bff-01" {
		t.Errorf("NameFromRequest() = %q, want bff-01", got)
	}
}

// TestTLSHandshake は実際の TLS 接続で、登録済みの証明書だけを受け付けることを確認する（D-13 §9）。
func TestTLSHandshake(t *testing.T) {
	serverCert := validKeyPair(t, "provisioning-api")
	registered := validKeyPair(t, "bff-01")
	var logBuf bytes.Buffer
	v := &Verifier{Clients: Clients{Fingerprint(registered.Leaf): "bff-01"}, Log: slog.New(slog.NewJSONHandler(&logBuf, nil))}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, v.Clients.NameFromRequest(r))
	}))
	srv.TLS = v.TLSConfig(serverCert)
	srv.Config.ErrorLog = slog.NewLogLogger(slog.NewJSONHandler(io.Discard, nil), slog.LevelDebug)
	srv.StartTLS()
	defer srv.Close()

	roots := x509.NewCertPool()
	roots.AddCert(serverCert.Leaf)
	client := func(certs ...tls.Certificate) *http.Client {
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
			RootCAs: roots, ServerName: "localhost", Certificates: certs,
		}}}
	}

	resp, err := client(registered).Get(srv.URL)
	if err != nil {
		t.Fatalf("registered client: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "bff-01" {
		t.Errorf("mgmt client = %q", body)
	}

	if _, err := client(validKeyPair(t, "other")).Get(srv.URL); err == nil {
		t.Error("unregistered client connected")
	}
	if _, err := client().Get(srv.URL); err == nil {
		t.Error("client without certificate connected")
	}
	// 拒否したログに送信元IPを残す
	if n := strings.Count(logBuf.String(), `"src_ip":"127.0.0.1"`); n != 2 {
		t.Errorf("rejection logs with src_ip = %d, want 2: %s", n, logBuf.String())
	}

	// TLS 1.1 以下は受け付けない
	old := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs: roots, ServerName: "localhost", Certificates: []tls.Certificate{registered}, MaxVersion: tls.VersionTLS11,
	}}}
	if _, err := old.Get(srv.URL); err == nil {
		t.Error("TLS 1.1 client connected")
	}
}
