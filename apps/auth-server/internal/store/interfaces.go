package store

import "context"

// ClientStore はRADIUSクライアントデータへのアクセスを定義する
type ClientStore interface {
	// GetClientSecret は指定されたIPのShared Secretを取得する
	// 未登録の場合は空文字列とnilを返す
	GetClientSecret(ctx context.Context, ip string) (string, error)
}
