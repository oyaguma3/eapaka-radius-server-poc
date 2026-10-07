// Package config は環境変数から設定を読み込む（D-13 §8.1.1）。
package config

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/kelseyhightower/envconfig"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/auth"
)

// Config は Provisioning API の設定を保持する。
type Config struct {
	// サーバー設定
	ListenAddr string `envconfig:"PROVISIONING_API_LISTEN_ADDR" default:":9444"`
	TLSCert    string `envconfig:"PROVISIONING_API_TLS_CERT" required:"true"`
	TLSKey     string `envconfig:"PROVISIONING_API_TLS_KEY" required:"true"`
	// AdminClientsRaw は管理クライアントの「識別名=フィンガープリント」のカンマ区切り
	AdminClientsRaw string `envconfig:"PROVISIONING_API_ADMIN_CLIENTS" required:"true"`
	NodeName        string `envconfig:"PROVISIONING_API_NODE_NAME"`
	GinMode         string `envconfig:"GIN_MODE" default:"release"`

	// Valkey設定
	RedisHost string `envconfig:"REDIS_HOST" default:"valkey"`
	RedisPort string `envconfig:"REDIS_PORT" default:"6379"`
	RedisPass string `envconfig:"REDIS_PASS"`

	// ログ設定
	LogLevel    string `envconfig:"LOG_LEVEL" default:"INFO"`
	LogMaskIMSI bool   `envconfig:"LOG_MASK_IMSI" default:"true"`

	// AdminClients は AdminClientsRaw を解釈した管理クライアント
	AdminClients auth.Clients `ignored:"true"`
}

// Load は環境変数から設定を読み込む。
// 管理クライアントが1件もない場合や、証明書のパスが空の場合はエラーを返す（起動しない）。
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.TLSCert == "" || cfg.TLSKey == "" {
		return nil, errors.New("PROVISIONING_API_TLS_CERT and PROVISIONING_API_TLS_KEY must not be empty")
	}

	clients, err := auth.ParseClients(cfg.AdminClientsRaw)
	if err != nil {
		return nil, fmt.Errorf("PROVISIONING_API_ADMIN_CLIENTS: %w", err)
	}
	if len(clients) == 0 {
		return nil, errors.New("PROVISIONING_API_ADMIN_CLIENTS: at least one admin client is required")
	}
	cfg.AdminClients = clients

	if cfg.NodeName == "" {
		cfg.NodeName, _ = os.Hostname()
	}
	return &cfg, nil
}

// RedisAddr は Valkey の接続先を返す。
func (c *Config) RedisAddr() string {
	return net.JoinHostPort(c.RedisHost, c.RedisPort)
}
