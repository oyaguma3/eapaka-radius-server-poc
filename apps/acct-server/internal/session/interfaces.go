package session

import "context"

// SessionManager はセッション状態管理のインターフェース
type SessionManager interface {
	// Exists はセッションの存在を確認する
	Exists(ctx context.Context, uuid string) (bool, error)
	// Get はセッション情報を取得する
	Get(ctx context.Context, uuid string) (*Session, error)
	// UpdateOnStart はStart受信時のセッション更新を行う
	UpdateOnStart(ctx context.Context, uuid string, data *SessionStartData) error
	// UpdateOnInterim はInterim受信時のセッション更新を行う
	UpdateOnInterim(ctx context.Context, uuid string, data *SessionInterimData) error
	// Delete はセッションを削除する
	Delete(ctx context.Context, uuid string) error
	// RemoveUserIndex はユーザーインデックスからセッションを削除する
	RemoveUserIndex(ctx context.Context, imsi, uuid string) error
}

// IdentifierResolver はログ出力用の識別子を解決するインターフェース
type IdentifierResolver interface {
	// ResolveIMSI はログ出力用のIMSI/識別子を取得する
	ResolveIMSI(ctx context.Context, sessionUUID, userName, classUUID string) string
}
