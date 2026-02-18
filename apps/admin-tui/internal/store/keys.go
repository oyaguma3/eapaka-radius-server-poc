// Package store はValkeyアクセス層を提供する。
package store

// キープレフィックス定義
const (
	// PrefixSubscriber は加入者キーのプレフィックス
	PrefixSubscriber = "sub:"
	// PrefixClient はRADIUSクライアントキーのプレフィックス
	PrefixClient = "client:"
	// PrefixPolicy は認可ポリシーキーのプレフィックス
	PrefixPolicy = "policy:"
	// PrefixSession はセッションキーのプレフィックス
	PrefixSession = "sess:"
	// PrefixEAPContext はEAPコンテキストキーのプレフィックス
	PrefixEAPContext = "eap:"
	// PrefixUserIndex はユーザーインデックスキーのプレフィックス
	PrefixUserIndex = "idx:user:"
	// KeyStatistics は統計情報キー
	KeyStatistics = "stats:global"
)

// SubscriberKey は加入者のValkeyキーを生成する。
func SubscriberKey(imsi string) string {
	return PrefixSubscriber + imsi
}

// ClientKey はRADIUSクライアントのValkeyキーを生成する。
func ClientKey(ip string) string {
	return PrefixClient + ip
}

// PolicyKey は認可ポリシーのValkeyキーを生成する。
func PolicyKey(imsi string) string {
	return PrefixPolicy + imsi
}

// SessionKey はセッションのValkeyキーを生成する。
func SessionKey(uuid string) string {
	return PrefixSession + uuid
}

// UserIndexKey はユーザーインデックスのValkeyキーを生成する。
func UserIndexKey(imsi string) string {
	return PrefixUserIndex + imsi
}
