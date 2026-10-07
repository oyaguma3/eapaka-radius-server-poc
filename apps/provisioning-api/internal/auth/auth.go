// Package auth は mTLS による管理クライアントの認証を提供する（D-13 §5）。
// クライアント証明書の SHA-256 フィンガープリントを、静的に設定した管理クライアントと照合する。
// CA による検証は行わない（aka-only-server の管理API と同じ方式）。
package auth

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var (
	clientNamePattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Clients は管理クライアントの一覧。フィンガープリント（SHA-256、16進小文字）から識別名を引く。
type Clients map[string]string

// Fingerprint は証明書（DER）の SHA-256 フィンガープリントを16進小文字で返す。
func Fingerprint(cert *x509.Certificate) string {
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// ParseClients は「識別名=フィンガープリント」のカンマ区切りを解釈する。
// フィンガープリントは大文字やコロン区切り（openssl の出力形式）も受け付ける。
func ParseClients(v string) (Clients, error) {
	clients := Clients{}
	for entry := range strings.SplitSeq(v, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, fp, ok := strings.Cut(entry, "=")
		name = strings.TrimSpace(name)
		if !ok || !clientNamePattern.MatchString(name) {
			return nil, fmt.Errorf("bad entry %q: want <name>=<sha256 fingerprint>", entry)
		}
		fp = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(fp), ":", ""))
		if !fingerprintPattern.MatchString(fp) {
			return nil, fmt.Errorf("bad fingerprint for %q: want 64 hex digits", name)
		}
		if other, dup := clients[fp]; dup {
			return nil, fmt.Errorf("fingerprint of %q is already used by %q", name, other)
		}
		clients[fp] = name
	}
	return clients, nil
}

// NameOf は証明書に対応する識別名を返す。登録されていなければ空文字と false を返す。
func (c Clients) NameOf(cert *x509.Certificate) (string, bool) {
	name, ok := c[Fingerprint(cert)]
	return name, ok
}

// NameFromRequest はリクエストのクライアント証明書に対応する識別名を返す。
// TLS でない、または証明書がない場合は空文字を返す。
func (c Clients) NameFromRequest(r *http.Request) string {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return ""
	}
	name, _ := c.NameOf(r.TLS.PeerCertificates[0])
	return name
}

// Verifier は TLS ハンドシェイクでクライアント証明書を照合する。
type Verifier struct {
	Clients Clients
	Log     *slog.Logger
	// Now は現在時刻を返す（テスト用）。nil なら time.Now。
	Now func() time.Time
}

// VerifyConnection は tls.Config.VerifyConnection に使う。
// 証明書がない、登録されていない、有効期間外のいずれかなら拒否する（HTTP の応答は返さない）。
func (v *Verifier) VerifyConnection(cs tls.ConnectionState) error {
	return v.verify(cs, "")
}

// verify は VerifyConnection の本体。srcIP は拒否したときのログに使う送信元IP（不明なら空文字）。
func (v *Verifier) verify(cs tls.ConnectionState, srcIP string) error {
	if len(cs.PeerCertificates) == 0 {
		v.reject("no client certificate", "", srcIP)
		return errors.New("client certificate required")
	}
	cert := cs.PeerCertificates[0]
	fp := Fingerprint(cert)
	if _, ok := v.Clients[fp]; !ok {
		v.reject("not configured", fp, srcIP)
		return errors.New("client certificate is not a configured admin client")
	}
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	if t := now(); t.Before(cert.NotBefore) || t.After(cert.NotAfter) {
		v.reject("outside validity period", fp, srcIP)
		return errors.New("client certificate is outside its validity period")
	}
	return nil
}

// reject は拒否した接続をログに残す（未登録の証明書を登録するときにフィンガープリントを確かめられるようにする）。
func (v *Verifier) reject(reason, fp, srcIP string) {
	if v.Log == nil {
		return
	}
	v.Log.Warn("admin client certificate rejected",
		"event_id", "PROV_CLIENT_REJECTED",
		"reason", reason,
		"fingerprint", fp,
		"src_ip", srcIP,
	)
}

// TLSConfig は provisioning-api のリスナーの TLS 設定を返す（TLS 1.2 以上、mTLS 必須）。
// クライアント証明書の有無も VerifyConnection で確認するため、ClientAuth は RequestClientCert にする
// （RequireAnyClientCert では証明書がない接続が VerifyConnection の前に拒否され、ログに残らない）。
// 提示された証明書の秘密鍵の所持（CertificateVerify）は、ClientAuth の値によらず検証される。
// 拒否のログに送信元IPを残すため、接続ごとに VerifyConnection を差し替えた設定を使う。
func (v *Verifier) TLSConfig(cert tls.Certificate) *tls.Config {
	base := &tls.Config{
		MinVersion:       tls.VersionTLS12,
		ClientAuth:       tls.RequestClientCert,
		Certificates:     []tls.Certificate{cert},
		VerifyConnection: v.VerifyConnection,
	}
	cfg := base.Clone()
	cfg.GetConfigForClient = func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		srcIP := ""
		if hello.Conn != nil {
			srcIP = hostOf(hello.Conn.RemoteAddr())
		}
		c := base.Clone()
		c.VerifyConnection = func(cs tls.ConnectionState) error { return v.verify(cs, srcIP) }
		return c, nil
	}
	return cfg
}

// hostOf はアドレスからホスト部分を取り出す。
func hostOf(addr net.Addr) string {
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}
