package valkey

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient は新しいValkeyクライアントを生成する。
// 接続確認のためPINGを実行し、失敗した場合はエラーを返す。
func NewClient(opts *Options) (*redis.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), opts.ConnectTimeout)
	defer cancel()
	return NewClientWithContext(ctx, opts)
}

// NewClientWithContext は指定されたコンテキストでValkeyクライアントを生成する。
func NewClientWithContext(ctx context.Context, opts *Options) (*redis.Client, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	client := redis.NewClient(&redis.Options{
		Addr:         opts.Addr,
		Password:     opts.Password,
		DB:           opts.DB,
		DialTimeout:  opts.ConnectTimeout,
		ReadTimeout:  opts.ReadTimeout,
		WriteTimeout: opts.WriteTimeout,
		PoolSize:     opts.PoolSize,
		MinIdleConns: opts.MinIdleConns,
	})

	// 接続確認
	if err := client.Ping(ctx).Err(); err != nil {
		// クリーンアップ
		_ = client.Close()
		return nil, err
	}

	return client, nil
}

// MustNewClient は新しいValkeyクライアントを生成する。
// 接続に失敗した場合はパニックする。
func MustNewClient(opts *Options) *redis.Client {
	client, err := NewClient(opts)
	if err != nil {
		panic(err)
	}
	return client
}

// IsConnectionError は接続関連のエラーかどうかを判定する。
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// タイムアウトエラー
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout()
	}

	// 接続拒否など
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// コンテキストエラー
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}

	return false
}

// IsKeyNotFound はキーが見つからないエラーかどうかを判定する。
func IsKeyNotFound(err error) bool {
	return errors.Is(err, redis.Nil)
}

// DefaultPingInterval はヘルスチェック用のデフォルトPING間隔。
const DefaultPingInterval = 30 * time.Second
