package session

import (
	"context"
	"fmt"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/store"
)

// EAPContext はEAP認証コンテキストを表す（D-09 セクション6.6.1/9.2.1準拠）。
type EAPContext struct {
	IMSI                 string `redis:"imsi"`
	Stage                string `redis:"stage"`
	EAPType              uint8  `redis:"eap_type"`
	RAND                 string `redis:"rand"`
	AUTN                 string `redis:"autn"`
	XRES                 string `redis:"xres"`
	Kaut                 string `redis:"k_aut"`
	MSK                  string `redis:"msk"`
	ResyncCount          int    `redis:"resync_count"`
	PermanentIDRequested bool   `redis:"permanent_id_requested"`
}

// contextStore はContextStoreの実装。
type contextStore struct {
	vc *store.ValkeyClient
}

// NewContextStore はContextStoreの新しいインスタンスを生成する。
func NewContextStore(vc *store.ValkeyClient) ContextStore {
	return &contextStore{vc: vc}
}

// Create はEAPコンテキストをValkeyに保存する。
func (s *contextStore) Create(ctx context.Context, traceID string, eapCtx *EAPContext) error {
	key := store.KeyPrefixEAPContext + traceID
	m := store.StructToMap(eapCtx)

	pipe := s.vc.Client().Pipeline()
	pipe.HSet(ctx, key, m)
	pipe.Expire(ctx, key, config.EAPContextTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("%w: %v", store.ErrValkeyUnavailable, err)
	}
	return nil
}

// Get はEAPコンテキストをValkeyから取得する。
func (s *contextStore) Get(ctx context.Context, traceID string) (*EAPContext, error) {
	key := store.KeyPrefixEAPContext + traceID
	m, err := s.vc.Client().HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrValkeyUnavailable, err)
	}
	if len(m) == 0 {
		return nil, ErrContextNotFound
	}

	var eapCtx EAPContext
	if err := store.MapToStruct(m, &eapCtx); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrContextInvalid, err)
	}
	return &eapCtx, nil
}

// Update はEAPコンテキストを部分更新し、TTLをリフレッシュする。
func (s *contextStore) Update(ctx context.Context, traceID string, updates map[string]any) error {
	key := store.KeyPrefixEAPContext + traceID

	exists, err := s.vc.Client().Exists(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("%w: %v", store.ErrValkeyUnavailable, err)
	}
	if exists == 0 {
		return ErrContextNotFound
	}

	pipe := s.vc.Client().Pipeline()
	pipe.HSet(ctx, key, updates)
	pipe.Expire(ctx, key, config.EAPContextTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("%w: %v", store.ErrValkeyUnavailable, err)
	}
	return nil
}

// Delete はEAPコンテキストを削除する。存在しなくてもエラーにしない。
func (s *contextStore) Delete(ctx context.Context, traceID string) error {
	key := store.KeyPrefixEAPContext + traceID
	if err := s.vc.Client().Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("%w: %v", store.ErrValkeyUnavailable, err)
	}
	return nil
}

// Exists はEAPコンテキストの存在を確認する。
func (s *contextStore) Exists(ctx context.Context, traceID string) (bool, error) {
	key := store.KeyPrefixEAPContext + traceID
	n, err := s.vc.Client().Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("%w: %v", store.ErrValkeyUnavailable, err)
	}
	return n > 0, nil
}
