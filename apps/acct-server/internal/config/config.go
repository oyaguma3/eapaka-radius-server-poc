package config

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

// Config はアプリケーション設定を保持する
type Config struct {
	// Valkey接続設定
	RedisHost string `envconfig:"REDIS_HOST" required:"true"`
	RedisPort string `envconfig:"REDIS_PORT" required:"true"`
	RedisPass string `envconfig:"REDIS_PASS" required:"true"`

	// RADIUS設定
	RadiusSecret string `envconfig:"RADIUS_SECRET"`
	ListenAddr   string `envconfig:"LISTEN_ADDR" default:":1813"`

	// ログ設定
	LogMaskIMSI bool `envconfig:"LOG_MASK_IMSI" default:"true"`
}

// Load は環境変数から設定を読み込む
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &cfg, nil
}

// ValkeyAddr はValkey接続アドレスを "host:port" 形式で返す
func (c *Config) ValkeyAddr() string {
	return fmt.Sprintf("%s:%s", c.RedisHost, c.RedisPort)
}
