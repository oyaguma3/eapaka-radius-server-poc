// Package store はValkeyアクセス層を提供する。
package store

// キープレフィックス定義（加入者・RADIUSクライアント・認可ポリシーのキーは pkg/masterdata）
const (
	// PrefixSession はセッションキーのプレフィックス
	PrefixSession = "sess:"
	// PrefixEAPContext はEAPコンテキストキーのプレフィックス
	PrefixEAPContext = "eap:"
	// PrefixUserIndex はユーザーインデックスキーのプレフィックス
	PrefixUserIndex = "idx:user:"
	// KeyStatistics は統計情報キー
	KeyStatistics = "stats:global"
)

// SessionKey はセッションのValkeyキーを生成する。
func SessionKey(uuid string) string {
	return PrefixSession + uuid
}

// UserIndexKey はユーザーインデックスのValkeyキーを生成する。
func UserIndexKey(imsi string) string {
	return PrefixUserIndex + imsi
}
