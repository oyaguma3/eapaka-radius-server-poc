package server

import (
	"context"
	"log/slog"
	"net"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/store"
)

// DynamicSecretSource はValkey登録情報に基づくRADIUS Secret解決を行う。
// layeh.com/radius.SecretSourceインターフェースの実装。
type DynamicSecretSource struct {
	clientStore    store.ClientStore
	fallbackSecret []byte
}

// NewSecretSource は新しいDynamicSecretSourceを生成する。
func NewSecretSource(cs store.ClientStore, fallbackSecret string) *DynamicSecretSource {
	var fb []byte
	if fallbackSecret != "" {
		fb = []byte(fallbackSecret)
	}
	return &DynamicSecretSource{
		clientStore:    cs,
		fallbackSecret: fb,
	}
}

// RADIUSSecret はリモートアドレスに対応するRADIUS Secretを返す。
func (s *DynamicSecretSource) RADIUSSecret(ctx context.Context, remoteAddr net.Addr) ([]byte, error) {
	ip := extractIP(remoteAddr)
	if ip == "" {
		if len(s.fallbackSecret) > 0 {
			return s.fallbackSecret, nil
		}
		return nil, nil
	}

	secret, err := s.clientStore.GetClientSecret(ctx, ip)
	if err != nil {
		slog.Warn("Valkeyクライアント検索エラー",
			"event_id", "RADIUS_SECRET_ERR",
			"src_ip", ip,
			"error", err,
		)
		if len(s.fallbackSecret) > 0 {
			return s.fallbackSecret, nil
		}
		return nil, nil
	}

	if secret != "" {
		return []byte(secret), nil
	}

	if len(s.fallbackSecret) > 0 {
		return s.fallbackSecret, nil
	}

	slog.Warn("RADIUS Secret不明",
		"event_id", "RADIUS_NO_SECRET",
		"src_ip", ip,
	)
	return nil, nil
}

// extractIP はnet.AddrからIPアドレス文字列を抽出する
func extractIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	if udpAddr, ok := addr.(*net.UDPAddr); ok {
		return udpAddr.IP.String()
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return ""
	}
	return host
}
