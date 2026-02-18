package aka

import (
	eapaka "github.com/oyaguma3/go-eapaka"
)

// KeyMaterial はEAP-AKA鍵導出結果を保持する
type KeyMaterial struct {
	K_encr []byte // 16バイト: 暗号化鍵
	K_aut  []byte // 16バイト: 認証鍵
	MSK    []byte // 64バイト: マスターセッション鍵
	EMSK   []byte // 64バイト: 拡張マスターセッション鍵
}

// DeriveKeys はEAP-AKA鍵導出を行う（RFC 4187 Section 7）
// eapaka.DeriveKeysAKA のラッパー
func DeriveKeys(identity string, ck, ik []byte) *KeyMaterial {
	keys := eapaka.DeriveKeysAKA(identity, ck, ik)
	return &KeyMaterial{
		K_encr: keys.K_encr,
		K_aut:  keys.K_aut,
		MSK:    keys.MSK,
		EMSK:   keys.EMSK,
	}
}
