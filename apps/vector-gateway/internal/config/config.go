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
	return &cfg, nil
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
