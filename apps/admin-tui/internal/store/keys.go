// Package store はValkeyアクセス層を提供する。
package store

import "github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"

// キープレフィックス定義（加入者・RADIUSクライアント・認可ポリシー・セッションのキーは pkg/masterdata）
const (
	// PrefixSession はセッションキーのプレフィックス
	PrefixSession = masterdata.PrefixSession
	// PrefixEAPContext はEAPコンテキストキーのプレフィックス
	PrefixEAPContext = "eap:"
	// PrefixUserIndex はユーザーインデックスキーのプレフィックス
	PrefixUserIndex = masterdata.PrefixUserIndex
	// KeyStatistics は統計情報キー
	KeyStatistics = "stats:global"
)

// SessionKey はセッションのValkeyキーを生成する。
func SessionKey(uuid string) string {
	return masterdata.SessionKey(uuid)
}

// UserIndexKey はユーザーインデックスのValkeyキーを生成する。
func UserIndexKey(imsi string) string {
	return masterdata.UserIndexKey(imsi)
}
