// Package store はValkeyへのデータアクセスを提供する。
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
	"github.com/redis/go-redis/v9"
)

// ValkeyClient はValkeyクライアントをラップする。
type ValkeyClient struct {
	client *redis.Client
}

// NewValkeyClient は新しいValkeyClientを生成する。
func NewValkeyClient(cfg *config.Config) (*ValkeyClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr(),
		Password:     cfg.RedisPass,
		DB:           0,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     10,
	})

	// 接続確認
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Valkey: %w", err)
	}

	return &ValkeyClient{client: client}, nil
}

// Close は接続を閉じる。
func (v *ValkeyClient) Close() error {
	return v.client.Close()
}

// Client は内部のredis.Clientを返す。
func (v *ValkeyClient) Client() *redis.Client {
	return v.client
}
