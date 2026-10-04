// Package config は環境変数から設定を読み込む。
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config はVector Gatewayの設定を保持する。
type Config struct {
	// Gateway動作モード（"gateway" or "passthrough"）
	Mode string `envconfig:"VECTOR_GATEWAY_MODE" default:"gateway"`

	// 内部Vector API接続先URL
	InternalURL string `envconfig:"VECTOR_GATEWAY_INTERNAL_URL" required:"true"`

	// 内部Vector APIへのタイムアウト
	InternalTimeout time.Duration `envconfig:"VECTOR_GATEWAY_INTERNAL_TIMEOUT" default:"5s"`

	// PLMNマッピング文字列（"44010:01,44020:01" 形式）
	PLMNMapRaw string `envconfig:"VECTOR_GATEWAY_PLMN_MAP" default:""`

	// aka-only-server（接続方式ID:01）のベースURL。空なら01を登録しない
	AKAOnlyURL string `envconfig:"VECTOR_GATEWAY_AKAONLY_URL" default:""`

	// aka-only-serverへのmTLSで使うクライアント証明書（PEM）。秘密鍵を同じファイルに含めてよい
	AKAOnlyClientCert string `envconfig:"VECTOR_GATEWAY_AKAONLY_CLIENT_CERT" default:""`

	// クライアント証明書の秘密鍵（PEM）。空ならAKAOnlyClientCertから読む
	AKAOnlyClientKey string `envconfig:"VECTOR_GATEWAY_AKAONLY_CLIENT_KEY" default:""`

	// aka-only-serverのAV用サーバー証明書（PEM）。これだけを信頼する
	AKAOnlyServerCert string `envconfig:"VECTOR_GATEWAY_AKAONLY_SERVER_CERT" default:""`

	// aka-only-serverへのタイムアウト
	AKAOnlyTimeout time.Duration `envconfig:"VECTOR_GATEWAY_AKAONLY_TIMEOUT" default:"5s"`

	// サーバー設定
	ListenAddr  string `envconfig:"LISTEN_ADDR" default:":8080"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"INFO"`
	LogMaskIMSI bool   `envconfig:"LOG_MASK_IMSI" default:"true"`
	GinMode     string `envconfig:"GIN_MODE" default:"release"`
}

// PLMNEntry はPLMNとバックエンドIDのマッピングを表す。
type PLMNEntry struct {
	PLMN      string
	BackendID string
}

// Load は環境変数から設定を読み込む。
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.validateAKAOnly(); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &cfg, nil
}

// validateAKAOnly はaka-only-server接続設定を検証し、URL末尾の"/"を取り除く。
// URLが空の場合は接続方式01を使わないため検証しない。
func (c *Config) validateAKAOnly() error {
	if c.AKAOnlyURL == "" {
		return nil
	}
	if !strings.HasPrefix(c.AKAOnlyURL, "http://") && !strings.HasPrefix(c.AKAOnlyURL, "https://") {
		return fmt.Errorf("VECTOR_GATEWAY_AKAONLY_URL must start with http:// or https://: %q", c.AKAOnlyURL)
	}
	c.AKAOnlyURL = strings.TrimRight(c.AKAOnlyURL, "/")
	if c.AKAOnlyTimeout <= 0 {
		return fmt.Errorf("VECTOR_GATEWAY_AKAONLY_TIMEOUT must be positive: %v", c.AKAOnlyTimeout)
	}
	if c.AKAOnlyUseTLS() {
		if c.AKAOnlyClientCert == "" {
			return fmt.Errorf("VECTOR_GATEWAY_AKAONLY_CLIENT_CERT is required for https")
		}
		if c.AKAOnlyServerCert == "" {
			return fmt.Errorf("VECTOR_GATEWAY_AKAONLY_SERVER_CERT is required for https")
		}
	}
	return nil
}

// AKAOnlyEnabled はaka-only-server（接続方式01）が設定されているかを返す。
func (c *Config) AKAOnlyEnabled() bool {
	return c.AKAOnlyURL != ""
}

// AKAOnlyUseTLS はaka-only-serverへmTLSで接続するかを返す。
func (c *Config) AKAOnlyUseTLS() bool {
	return strings.HasPrefix(c.AKAOnlyURL, "https://")
}

// ParsePLMNMap はPLMNマッピング文字列をパースしてマップを返す。
// 形式: "PLMN:BackendID,PLMN:BackendID" (例: "44010:01,44020:01")
// PLMNは5-6桁の数字、BackendIDは2桁の数字であること。
func (c *Config) ParsePLMNMap() (map[string]string, error) {
	result := make(map[string]string)

	if c.PLMNMapRaw == "" {
		return result, nil
	}

	entries := strings.Split(c.PLMNMapRaw, ",")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}

		parts := strings.SplitN(entry, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid PLMN map entry: %q (expected PLMN:BackendID)", entry)
		}

		plmn := strings.TrimSpace(parts[0])
		backendID := strings.TrimSpace(parts[1])

		// PLMNバリデーション: 5-6桁の数字
		if err := validatePLMN(plmn); err != nil {
			return nil, fmt.Errorf("invalid PLMN in map entry %q: %w", entry, err)
		}

		// BackendIDバリデーション: 2桁の数字
		if err := validateBackendID(backendID); err != nil {
			return nil, fmt.Errorf("invalid BackendID in map entry %q: %w", entry, err)
		}

		result[plmn] = backendID
	}

	return result, nil
}

// IsPassthrough はpassthroughモードかどうかを返す。
func (c *Config) IsPassthrough() bool {
	return c.Mode == "passthrough"
}

// validatePLMN はPLMNが5-6桁の数字であることを検証する。
func validatePLMN(plmn string) error {
	if len(plmn) < 5 || len(plmn) > 6 {
		return fmt.Errorf("PLMN must be 5-6 digits, got %d digits", len(plmn))
	}
	for _, c := range plmn {
		if c < '0' || c > '9' {
			return fmt.Errorf("PLMN must contain only digits")
		}
	}
	return nil
}

// validateBackendID はバックエンドIDが2桁の数字であることを検証する。
func validateBackendID(id string) error {
	if len(id) != 2 {
		return fmt.Errorf("BackendID must be 2 digits, got %d characters", len(id))
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return fmt.Errorf("BackendID must contain only digits")
		}
	}
	return nil
}
