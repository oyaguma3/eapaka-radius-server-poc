// Package config は環境変数から設定を読み込む。
package config

import (
	"fmt"
	"net"

	"github.com/kelseyhightower/envconfig"
)

// Config はVector APIの設定を保持する。
type Config struct {
	// Valkey設定
	RedisHost string `envconfig:"REDIS_HOST" required:"true"`
	RedisPort string `envconfig:"REDIS_PORT" required:"true"`
	RedisPass string `envconfig:"REDIS_PASS" required:"true"`

	// サーバー設定
	ListenAddr  string `envconfig:"LISTEN_ADDR" default:":8080"`
	LogLevel    string `envconfig:"LOG_LEVEL" default:"INFO"`
	LogMaskIMSI bool   `envconfig:"LOG_MASK_IMSI" default:"true"`
	GinMode     string `envconfig:"GIN_MODE" default:"release"`

	// テストモード設定
	TestVectorEnabled    bool   `envconfig:"TEST_VECTOR_ENABLED" default:"false"`
	TestVectorIMSIPrefix string `envconfig:"TEST_VECTOR_IMSI_PREFIX" default:"00101"`
}

// Load は環境変数から設定を読み込む。
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &cfg, nil
}

// RedisAddr はValkey接続文字列を返す。
func (c *Config) RedisAddr() string {
	return net.JoinHostPort(c.RedisHost, c.RedisPort)
}
