// Package masterdata は加入者・RADIUSクライアント・認可ポリシー（マスタデータ）の Valkey アクセス層を提供する。
// Admin TUI と Provisioning API が共通で使う（D-13 §7.2）。キーとフィールドの形式は D-02 に従う。
package masterdata

import "strconv"

// キープレフィックス定義
const (
	// PrefixSubscriber は加入者キーのプレフィックス
	PrefixSubscriber = "sub:"
	// PrefixClient はRADIUSクライアントキーのプレフィックス
	PrefixClient = "client:"
	// PrefixPolicy は認可ポリシーキーのプレフィックス
	PrefixPolicy = "policy:"
	// PrefixClientIndex はRADIUSクライアントのIDからIPを引く索引（String）のプレフィックス。
	// client:* の SCAN に含まれないよう、client: で始めない。
	PrefixClientIndex = "idx:client:"
	// KeyClientSeq はRADIUSクライアントのIDの採番に使うカウンター（String。INCR）。
	KeyClientSeq = "seq:client"
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

// ClientIndexKey はRADIUSクライアントのIDからIPを引く索引のキーを生成する。
func ClientIndexKey(id int64) string {
	return PrefixClientIndex + strconv.FormatInt(id, 10)
}
