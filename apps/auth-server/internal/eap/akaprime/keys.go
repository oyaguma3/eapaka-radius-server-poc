package akaprime

import (
	eapaka "github.com/oyaguma3/go-eapaka"
)

// KeyMaterial はEAP-AKA'鍵導出結果を保持する
type KeyMaterial struct {
	K_encr []byte // 16バイト: 暗号化鍵
	K_aut  []byte // 32バイト: 認証鍵（EAP-AKAの16Bと異なる）
	K_re   []byte // 32バイト: 再認証鍵
	MSK    []byte // 64バイト: マスターセッション鍵
	EMSK   []byte // 64バイト: 拡張マスターセッション鍵
}

// DeriveCKPrimeIKPrime はCK'/IK'を導出する（RFC 9048 Section 3.3）
func DeriveCKPrimeIKPrime(ck, ik []byte, networkName string, autn []byte) (ckPrime, ikPrime []byte, err error) {
	return eapaka.DeriveCKPrimeIKPrime(ck, ik, networkName, autn)
}

// DeriveKeys はEAP-AKA'セッション鍵導出を行う
// eapaka.DeriveKeysAKAPrime のラッパー
func DeriveKeys(identity string, ckPrime, ikPrime []byte) *KeyMaterial {
	keys := eapaka.DeriveKeysAKAPrime(identity, ckPrime, ikPrime)
	return &KeyMaterial{
		K_encr: keys.K_encr,
		K_aut:  keys.K_aut,
		K_re:   keys.K_re,
		MSK:    keys.MSK,
		EMSK:   keys.EMSK,
	}
}

// DeriveAllKeys はCK'/IK'導出とセッション鍵導出を統合的に行う
func DeriveAllKeys(identity string, ck, ik, autn []byte, networkName string) (*KeyMaterial, error) {
	ckPrime, ikPrime, err := DeriveCKPrimeIKPrime(ck, ik, networkName, autn)
	if err != nil {
		return nil, err
	}
	return DeriveKeys(identity, ckPrime, ikPrime), nil
}
