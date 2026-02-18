package radius

import (
	"crypto/hmac"
	"crypto/md5"
	"testing"

	radiuspkg "layeh.com/radius"
	"layeh.com/radius/rfc2869"
)

// createPacketWithMA はMessage-Authenticator付きのテスト用パケットを作成する
func createPacketWithMA(t *testing.T, secret []byte) *radiuspkg.Packet {
	t.Helper()
	packet := &radiuspkg.Packet{
		Code:       radiuspkg.CodeStatusServer,
		Identifier: 1,
		Secret:     secret,
	}

	// 16バイトゼロのMAをプレースホルダーとして設定
	zeroMA := make([]byte, 16)
	_ = rfc2869.MessageAuthenticator_Set(packet, zeroMA)

	// パケットをシリアライズしてHMAC-MD5を計算
	data, err := packet.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}

	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	computed := mac.Sum(nil)

	// 計算結果でMAを上書き
	_ = rfc2869.MessageAuthenticator_Set(packet, computed)

	return packet
}

func TestVerifyMessageAuthenticator_Valid(t *testing.T) {
	secret := []byte("testing123")
	packet := createPacketWithMA(t, secret)

	if !VerifyMessageAuthenticator(packet, secret) {
		t.Error("VerifyMessageAuthenticator should return true for valid MA")
	}
}

func TestVerifyMessageAuthenticator_Missing(t *testing.T) {
	secret := []byte("testing123")
	packet := &radiuspkg.Packet{
		Code:       radiuspkg.CodeStatusServer,
		Identifier: 1,
		Secret:     secret,
	}

	// Message-Authenticator属性なし
	if VerifyMessageAuthenticator(packet, secret) {
		t.Error("VerifyMessageAuthenticator should return false when MA is missing")
	}
}

func TestVerifyMessageAuthenticator_Invalid(t *testing.T) {
	secret := []byte("testing123")
	packet := createPacketWithMA(t, secret)

	// MAを改ざん
	ma, _ := rfc2869.MessageAuthenticator_Lookup(packet)
	ma[0] ^= 0xFF
	_ = rfc2869.MessageAuthenticator_Set(packet, ma)

	if VerifyMessageAuthenticator(packet, secret) {
		t.Error("VerifyMessageAuthenticator should return false for invalid MA")
	}
}

func TestVerifyMessageAuthenticator_WrongSecret(t *testing.T) {
	secret := []byte("testing123")
	wrongSecret := []byte("wrong")
	packet := createPacketWithMA(t, secret)

	if VerifyMessageAuthenticator(packet, wrongSecret) {
		t.Error("VerifyMessageAuthenticator should return false for wrong secret")
	}
}

func TestVerifyMessageAuthenticator_ShortMA(t *testing.T) {
	secret := []byte("testing123")
	packet := &radiuspkg.Packet{
		Code:       radiuspkg.CodeStatusServer,
		Identifier: 1,
		Secret:     secret,
	}

	// 不正な長さのMAを設定（15バイト）
	shortMA := make([]byte, 15)
	_ = rfc2869.MessageAuthenticator_Set(packet, shortMA)

	if VerifyMessageAuthenticator(packet, secret) {
		t.Error("VerifyMessageAuthenticator should return false for short MA")
	}
}

func TestSetMessageAuthenticator(t *testing.T) {
	secret := []byte("testing123")
	requestAuth := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	// 応答パケットを作成
	response := &radiuspkg.Packet{
		Code:       radiuspkg.CodeAccountingResponse,
		Identifier: 1,
		Secret:     secret,
	}

	// SetMessageAuthenticatorを呼び出し
	SetMessageAuthenticator(response, secret, requestAuth)

	// MAが設定されたことを確認
	ma, err := rfc2869.MessageAuthenticator_Lookup(response)
	if err != nil {
		t.Fatalf("MessageAuthenticator not set: %v", err)
	}
	if len(ma) != 16 {
		t.Errorf("MA length = %d, want 16", len(ma))
	}

	// MAがすべてゼロでないことを確認
	allZero := true
	for _, b := range ma {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("MA should not be all zeros")
	}
}

func TestSetMessageAuthenticator_VerifyResult(t *testing.T) {
	secret := []byte("testing123")

	// リクエストパケットを作成
	request := &radiuspkg.Packet{
		Code:       radiuspkg.CodeStatusServer,
		Identifier: 1,
		Secret:     secret,
	}

	// リクエスト側にMAを設定（正しいMAを生成）
	zeroMA := make([]byte, 16)
	_ = rfc2869.MessageAuthenticator_Set(request, zeroMA)
	data, _ := request.MarshalBinary()
	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	_ = rfc2869.MessageAuthenticator_Set(request, mac.Sum(nil))

	// 応答パケットを作成
	response := request.Response(radiuspkg.CodeAccountingResponse)

	// SetMessageAuthenticatorを呼び出し
	SetMessageAuthenticator(response, secret, request.Authenticator)

	// MAが設定されたことを確認
	ma, err := rfc2869.MessageAuthenticator_Lookup(response)
	if err != nil {
		t.Fatalf("MessageAuthenticator not set: %v", err)
	}
	if len(ma) != 16 {
		t.Errorf("MA length = %d, want 16", len(ma))
	}
}
