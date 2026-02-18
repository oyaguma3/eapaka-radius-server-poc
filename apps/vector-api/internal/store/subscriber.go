package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const (
	maxRetries    = 2
	retryInterval = 100 * time.Millisecond
)

// Subscriber は加入者情報を表す。
type Subscriber struct {
	IMSI string
	Ki   string // Hex 32桁
	OPc  string // Hex 32桁
	AMF  string // Hex 4桁
	SQN  string // Hex 12桁
}

// SubscriberStore は加入者データへのアクセスを提供する。
type SubscriberStore struct {
	client *ValkeyClient
}

// NewSubscriberStore は新しいSubscriberStoreを生成する。
func NewSubscriberStore(client *ValkeyClient) *SubscriberStore {
	return &SubscriberStore{client: client}
}

// Get は加入者情報を取得する。
// キー: sub:{IMSI}
func (s *SubscriberStore) Get(ctx context.Context, imsi string) (*Subscriber, error) {
	key := "sub:" + imsi

	result, err := s.client.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %w", err)
	}

	if len(result) == 0 {
		return nil, nil // 未登録
	}

	return &Subscriber{
		IMSI: imsi,
		Ki:   result["ki"],
		OPc:  result["opc"],
		AMF:  result["amf"],
		SQN:  result["sqn"],
	}, nil
}

// UpdateSQN は加入者のSQNを更新する。
func (s *SubscriberStore) UpdateSQN(ctx context.Context, imsi string, sqn string) error {
	key := "sub:" + imsi

	err := s.client.client.HSet(ctx, key, "sqn", sqn).Err()
	if err != nil {
		return fmt.Errorf("failed to update SQN: %w", err)
	}

	return nil
}

// GetWithRetry はリトライ付きで加入者情報を取得する。
func (s *SubscriberStore) GetWithRetry(ctx context.Context, imsi string) (*Subscriber, error) {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		sub, err := s.Get(ctx, imsi)
		if err == nil {
			return sub, nil
		}

		lastErr = err

		// 接続エラーの場合のみリトライ
		if !isConnectionError(err) {
			return nil, err
		}

		if i < maxRetries {
			slog.Warn("Valkey connection failed, retrying",
				"event_id", "VALKEY_CONN_ERR",
				"retry", i+1,
				"error", err.Error(),
			)
			time.Sleep(retryInterval)
		}
	}

	return nil, lastErr
}

// isConnectionError は接続エラーかどうかを判定する。
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "connection reset")
}
