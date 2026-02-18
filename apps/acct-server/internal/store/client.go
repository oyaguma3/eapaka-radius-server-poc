package store

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// clientStore はClientStoreインターフェースの実装。
type clientStore struct {
	vc *ValkeyClient
}

// NewClientStore は新しいClientStoreを生成する。
func NewClientStore(vc *ValkeyClient) ClientStore {
	return &clientStore{vc: vc}
}

// GetClientSecret は指定されたIPのShared Secretを取得する。
// 未登録の場合は空文字列とnilを返す。
func (s *clientStore) GetClientSecret(ctx context.Context, ip string) (string, error) {
	key := KeyPrefixClient + ip
	secret, err := s.vc.Client().HGet(ctx, key, "secret").Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}
	return secret, nil
}
