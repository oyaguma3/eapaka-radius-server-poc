package radius

import (
	"crypto/md5"
	"testing"

	radiuspkg "layeh.com/radius"
)

func TestVerifyAccountingAuthenticator(t *testing.T) {
	secret := []byte("testing123")

	// テスト用パケットを構築
	packet := &radiuspkg.Packet{
		Code:       radiuspkg.CodeAccountingRequest,
		Identifier: 1,
		Secret:     secret,
	}

	// 正しいAuthenticatorを計算して設定
	data, err := packet.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}
	// Authenticatorフィールド（オフセット4-19）をゼロに
	copy(data[4:20], make([]byte, 16))
	h := md5.New()
	h.Write(data)
	h.Write(secret)
	copy(packet.Authenticator[:], h.Sum(nil))

	// 正しいAuthenticatorで検証
	if !VerifyAccountingAuthenticator(packet, secret) {
		t.Error("VerifyAccountingAuthenticator should return true for valid authenticator")
	}

	// 不正なAuthenticatorで検証
	packet.Authenticator[0] ^= 0xFF
	if VerifyAccountingAuthenticator(packet, secret) {
		t.Error("VerifyAccountingAuthenticator should return false for invalid authenticator")
	}
}

func TestVerifyAccountingAuthenticator_WrongSecret(t *testing.T) {
	secret := []byte("testing123")
	wrongSecret := []byte("wrong")

	packet := &radiuspkg.Packet{
		Code:       radiuspkg.CodeAccountingRequest,
		Identifier: 1,
		Secret:     secret,
	}

	// 正しいsecretでAuthenticatorを計算
	data, err := packet.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}
	copy(data[4:20], make([]byte, 16))
	h := md5.New()
	h.Write(data)
	h.Write(secret)
	copy(packet.Authenticator[:], h.Sum(nil))

	// 異なるsecretで検証
	if VerifyAccountingAuthenticator(packet, wrongSecret) {
		t.Error("VerifyAccountingAuthenticator should return false for wrong secret")
	}
}
