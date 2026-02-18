package store

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Store はValkeyへのアクセスを提供する。
type Store struct {
	client *redis.Client
}

// New は新しいStoreを生成する。
func New(client *redis.Client) *Store {
	return &Store{client: client}
}

// Client は内部のRedisクライアントを返す。
func (s *Store) Client() *redis.Client {
	return s.client
}

// Ping は接続確認を行う。
func (s *Store) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

// Close は接続をクローズする。
func (s *Store) Close() error {
	return s.client.Close()
}
