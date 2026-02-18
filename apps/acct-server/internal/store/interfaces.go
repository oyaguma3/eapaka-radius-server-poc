package store

import "context"

// ClientStore はRADIUSクライアントデータへのアクセスを定義する
type ClientStore interface {
	// GetClientSecret は指定されたIPのShared Secretを取得する
	// 未登録の場合は空文字列とnilを返す
	GetClientSecret(ctx context.Context, ip string) (string, error)
}

// SessionStore はAccounting用セッションデータへのアクセスを定義する
type SessionStore interface {
	// Exists はセッションの存在を確認する
	Exists(ctx context.Context, uuid string) (bool, error)
	// Get はセッション情報を取得する
	Get(ctx context.Context, uuid string) (map[string]string, error)
	// UpdateOnStart はStart受信時のセッション更新を行う
	UpdateOnStart(ctx context.Context, uuid string, fields map[string]any) error
	// UpdateOnInterim はInterim受信時のセッション更新を行う
	UpdateOnInterim(ctx context.Context, uuid string, fields map[string]any) error
	// Delete はセッションを削除する
	Delete(ctx context.Context, uuid string) error
	// RemoveUserIndex はユーザーインデックスからセッションを削除する
	RemoveUserIndex(ctx context.Context, imsi, uuid string) error
}

// DuplicateStore は重複検出用のValkey操作を定義する
type DuplicateStore interface {
	// Get は指定キーの値を取得する（未存在時は空文字列とnilを返す）
	Get(ctx context.Context, acctSessionID string) (string, error)
	// Set は指定キーに値を設定する
	Set(ctx context.Context, acctSessionID, value string) error
}
