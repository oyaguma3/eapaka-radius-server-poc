package server

import (
	"context"
	"log/slog"
	"net"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/store"
)

// DynamicSecretSource はValkey登録情報に基づくRADIUS Secret解決を行う。
// layeh.com/radius.SecretSourceインターフェースの実装。
type DynamicSecretSource struct {
	clientStore    store.ClientStore
	fallbackSecret []byte
}

// NewSecretSource は新しいDynamicSecretSourceを生成する。
// fallbackSecretが空文字列の場合、フォールバックは無効。
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
// Valkey登録 → フォールバック → nilの優先順で解決する。
func (s *DynamicSecretSource) RADIUSSecret(ctx context.Context, remoteAddr net.Addr) ([]byte, error) {
	ip := extractIP(remoteAddr)
	if ip == "" {
		var addrStr string
		if remoteAddr != nil {
			addrStr = remoteAddr.String()
		}
		slog.Warn("IPアドレス抽出失敗",
			"event_id", "RADIUS_IP_EXTRACT_ERR",
			"remote_addr", addrStr,
		)
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
		// Valkeyエラー時はフォールバック
		if len(s.fallbackSecret) > 0 {
			return s.fallbackSecret, nil
		}
		return nil, nil
	}

	if secret != "" {
		return []byte(secret), nil
	}

	// Valkey未登録
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
	// UDPAddr以外の場合はhost部分を試行
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return ""
	}
	return host
}
