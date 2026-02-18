package radius

import (
	"crypto/hmac"
	"crypto/md5"
	"testing"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2869"
)

func TestVerifyMessageAuthenticator_Valid(t *testing.T) {
	secret := []byte("testing-secret")
	p := radius.New(radius.CodeAccessRequest, secret)
	_ = rfc2865.UserName_AddString(p, "testuser")
	_ = rfc2869.EAPMessage_Set(p, []byte{0x01, 0x00, 0x00, 0x04})

	// 正しいMessage-Authenticatorを計算して設定
	zeroMA := make([]byte, 16)
	_ = rfc2869.MessageAuthenticator_Set(p, zeroMA)
	data, err := p.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	correctMA := mac.Sum(nil)
	_ = rfc2869.MessageAuthenticator_Set(p, correctMA)

	if !VerifyMessageAuthenticator(p, secret) {
		t.Error("VerifyMessageAuthenticator returned false for valid MA")
	}
}

func TestVerifyMessageAuthenticator_Invalid(t *testing.T) {
	secret := []byte("testing-secret")
	p := radius.New(radius.CodeAccessRequest, secret)
	_ = rfc2865.UserName_AddString(p, "testuser")
	_ = rfc2869.EAPMessage_Set(p, []byte{0x01, 0x00, 0x00, 0x04})

	// 不正なMessage-Authenticatorを設定
	invalidMA := make([]byte, 16)
	invalidMA[0] = 0xFF
	_ = rfc2869.MessageAuthenticator_Set(p, invalidMA)

	if VerifyMessageAuthenticator(p, secret) {
		t.Error("VerifyMessageAuthenticator returned true for invalid MA")
	}
}

func TestVerifyMessageAuthenticator_Missing(t *testing.T) {
	secret := []byte("testing-secret")
	p := radius.New(radius.CodeAccessRequest, secret)
	_ = rfc2865.UserName_AddString(p, "testuser")

	// Message-Authenticator属性なし
	if VerifyMessageAuthenticator(p, secret) {
		t.Error("VerifyMessageAuthenticator returned true for missing MA")
	}
}

func TestSetMessageAuthenticator_Roundtrip(t *testing.T) {
	secret := []byte("roundtrip-secret")
	reqPacket := radius.New(radius.CodeAccessRequest, secret)
	_ = rfc2865.UserName_AddString(reqPacket, "testuser")
	_ = rfc2869.EAPMessage_Set(reqPacket, []byte{0x01, 0x00, 0x00, 0x04})

	// リクエストにMessage-Authenticatorを設定して検証
	// リクエストの場合はrequestAuth = packet.Authenticator
	SetMessageAuthenticator(reqPacket, secret, reqPacket.Authenticator)

	if !VerifyMessageAuthenticator(reqPacket, secret) {
		t.Error("SetMessageAuthenticator → VerifyMessageAuthenticator roundtrip failed")
	}
}

func TestSetMessageAuthenticator_UsesRequestAuth(t *testing.T) {
	secret := []byte("auth-secret")
	reqPacket := radius.New(radius.CodeAccessRequest, secret)
	_ = rfc2869.EAPMessage_Set(reqPacket, []byte{0x01, 0x00, 0x00, 0x04})

	// レスポンスパケットを作成
	resp := reqPacket.Response(radius.CodeAccessAccept)
	_ = rfc2869.EAPMessage_Set(resp, []byte{0x03, 0x00, 0x00, 0x04})

	// Request Authenticatorを使用してMessage-Authenticatorを設定
	SetMessageAuthenticator(resp, secret, reqPacket.Authenticator)

	// 手動で検証: Request Authenticatorを使って計算
	ma, err := rfc2869.MessageAuthenticator_Lookup(resp)
	if err != nil {
		t.Fatal("Message-Authenticator not found after SetMessageAuthenticator")
	}

	// 同じ計算を再現
	zeroMA := make([]byte, 16)
	_ = rfc2869.MessageAuthenticator_Set(resp, zeroMA)
	savedAuth := resp.Authenticator
	resp.Authenticator = reqPacket.Authenticator
	data, err := resp.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	expected := mac.Sum(nil)
	resp.Authenticator = savedAuth

	if !hmac.Equal(ma, expected) {
		t.Error("SetMessageAuthenticator did not use request authenticator correctly")
	}
}
