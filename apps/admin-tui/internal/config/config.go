// Package config はAdmin TUIの設定管理を提供する。
package config

import (
	"os"
)

// Config はAdmin TUIの設定を表す。
type Config struct {
	ValkeyPassword string // Valkeyパスワード
	ValkeyAddr     string // Valkeyアドレス（固定値）
}

// Load は環境変数から設定を読み込む。
func Load() *Config {
	return &Config{
		ValkeyPassword: os.Getenv("VALKEY_PASSWORD"),
		ValkeyAddr:     "127.0.0.1:6379", // 固定値
	}
}
