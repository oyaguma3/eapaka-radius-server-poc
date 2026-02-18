package config

import "time"

// Valkey接続設定（D-01準拠）
const (
	ValkeyConnectTimeout = 3 * time.Second
	ValkeyCommandTimeout = 2 * time.Second
	ValkeyPoolSize       = 10
	ValkeyMaxRetries     = 3
	ValkeyMinRetryDelay  = 100 * time.Millisecond
	ValkeyMaxRetryDelay  = 1 * time.Second
)

// セッション管理（D-02準拠）
const (
	SessionTTL = 24 * time.Hour
)

// 重複検出TTL（D-10準拠）
const (
	DuplicateDetectTTL = 24 * time.Hour
)

// サーバーシャットダウン設定
const (
	ShutdownTimeout = 5 * time.Second
)
