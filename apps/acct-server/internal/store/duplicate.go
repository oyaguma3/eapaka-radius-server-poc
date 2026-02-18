package store

import (
	"context"
	"fmt"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/config"
	"github.com/redis/go-redis/v9"
)

// duplicateStore はDuplicateStoreインターフェースの実装。
type duplicateStore struct {
	vc *ValkeyClient
}

// NewDuplicateStore は新しいDuplicateStoreを生成する。
func NewDuplicateStore(vc *ValkeyClient) DuplicateStore {
	return &duplicateStore{vc: vc}
}

// Get は指定されたAcct-Session-IDの重複検出用の値を取得する。
// 未存在時は空文字列とnilを返す。
func (d *duplicateStore) Get(ctx context.Context, acctSessionID string) (string, error) {
	key := KeyPrefixAcctSeen + acctSessionID
	val, err := d.vc.Client().Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}
	return val, nil
}

// Set は指定されたAcct-Session-IDに重複検出用の値を設定する。
func (d *duplicateStore) Set(ctx context.Context, acctSessionID, value string) error {
	key := KeyPrefixAcctSeen + acctSessionID
	if err := d.vc.Client().Set(ctx, key, value, config.DuplicateDetectTTL).Err(); err != nil {
		return fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}
	return nil
}
