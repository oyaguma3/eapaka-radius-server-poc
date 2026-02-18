package config

import (
	"fmt"
	"strings"

	"github.com/kelseyhightower/envconfig"
)

// Config はアプリケーション設定を保持する
type Config struct {
	// Valkey接続設定
	RedisHost string `envconfig:"REDIS_HOST" required:"true"`
	RedisPort string `envconfig:"REDIS_PORT" required:"true"`
	RedisPass string `envconfig:"REDIS_PASS" required:"true"`

	// Vector Gateway設定
	VectorAPIURL string `envconfig:"VECTOR_API_URL" required:"true"`

	// RADIUS設定
	RadiusSecret string `envconfig:"RADIUS_SECRET"`
	ListenAddr   string `envconfig:"LISTEN_ADDR" default:":1812"`

	// EAP-AKA'設定
	NetworkName string `envconfig:"EAP_AKA_PRIME_NETWORK_NAME" default:"WLAN"`

	// ログ設定
	LogMaskIMSI bool `envconfig:"LOG_MASK_IMSI" default:"true"`
}

// Load は環境変数から設定を読み込む
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}
	return &cfg, nil
}

// ValkeyAddr はValkey接続アドレスを "host:port" 形式で返す
func (c *Config) ValkeyAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}

// validate は設定値のバリデーションを行う
func (c *Config) validate() error {
	if strings.TrimSpace(c.NetworkName) == "" {
		return fmt.Errorf("EAP_AKA_PRIME_NETWORK_NAME must not be empty")
	}
	if !strings.HasPrefix(c.VectorAPIURL, "http://") && !strings.HasPrefix(c.VectorAPIURL, "https://") {
		return fmt.Errorf("VECTOR_API_URL must start with http:// or https://")
	}
	return nil
}
