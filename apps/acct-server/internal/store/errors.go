package store

import "errors"

var (
	// ErrValkeyUnavailable はValkeyへの接続が利用不可能な場合のエラー
	ErrValkeyUnavailable = errors.New("valkey unavailable")

	// ErrKeyNotFound は指定されたキーが存在しない場合のエラー
	ErrKeyNotFound = errors.New("key not found")
)
