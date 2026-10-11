package config

import "time"

// Valkey接続設定（D-06準拠）
const (
	ValkeyConnectTimeout = 3 * time.Second
	ValkeyCommandTimeout = 2 * time.Second
	ValkeyPoolSize       = 10
)

// Vector Gateway接続設定（D-06準拠）。接続の確立も要求全体の上限に含める（接続だけの上限は設けない）
const (
	VectorRequestTimeout = 5 * time.Second
)

// Circuit Breaker設定（D-06準拠）
const (
	CBName             = "vector-gateway"
	CBMaxRequests      = 3
	CBInterval         = 10 * time.Second
	CBTimeout          = 30 * time.Second
	CBFailureThreshold = 5
)

// セッション管理（D-02準拠）
const (
	EAPContextTTL = 60 * time.Second
	SessionTTL    = 24 * time.Hour
)

// 再同期上限（D-02準拠）
const (
	MaxResyncCount = 32
)

// サーバーシャットダウン設定（D-09 3.9準拠）
const (
	ShutdownTimeout = 5 * time.Second
)
