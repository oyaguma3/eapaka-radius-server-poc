package store

import (
	"context"
	"fmt"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/config"
)

// sessionStore はSessionStoreインターフェースの実装。
type sessionStore struct {
	vc *ValkeyClient
}

// NewSessionStore は新しいSessionStoreを生成する。
func NewSessionStore(vc *ValkeyClient) SessionStore {
	return &sessionStore{vc: vc}
}

// Exists はセッションの存在を確認する。
func (s *sessionStore) Exists(ctx context.Context, uuid string) (bool, error) {
	key := KeyPrefixSession + uuid
	n, err := s.vc.Client().Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}
	return n > 0, nil
}

// Get はセッション情報を取得する。
func (s *sessionStore) Get(ctx context.Context, uuid string) (map[string]string, error) {
	key := KeyPrefixSession + uuid
	m, err := s.vc.Client().HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}
	if len(m) == 0 {
		return nil, ErrKeyNotFound
	}
	return m, nil
}

// UpdateOnStart はStart受信時のセッション更新を行う。
func (s *sessionStore) UpdateOnStart(ctx context.Context, uuid string, fields map[string]any) error {
	key := KeyPrefixSession + uuid
	pipe := s.vc.Client().Pipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, config.SessionTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}
	return nil
}

// UpdateOnInterim はInterim受信時のセッション更新を行う。
func (s *sessionStore) UpdateOnInterim(ctx context.Context, uuid string, fields map[string]any) error {
	key := KeyPrefixSession + uuid
	pipe := s.vc.Client().Pipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, config.SessionTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}
	return nil
}

// Delete はセッションを削除する。
func (s *sessionStore) Delete(ctx context.Context, uuid string) error {
	key := KeyPrefixSession + uuid
	if err := s.vc.Client().Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}
	return nil
}

// RemoveUserIndex はユーザーインデックスからセッションを削除する。
func (s *sessionStore) RemoveUserIndex(ctx context.Context, imsi, uuid string) error {
	key := KeyPrefixUserIndex + imsi
	if err := s.vc.Client().SRem(ctx, key, uuid).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}
	return nil
}
