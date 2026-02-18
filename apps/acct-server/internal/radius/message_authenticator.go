package radius

import (
	"crypto/hmac"
	"crypto/md5"

	"layeh.com/radius"
	"layeh.com/radius/rfc2869"
)

// VerifyMessageAuthenticator はMessage-Authenticator属性を検証する（Status-Server用）。
func VerifyMessageAuthenticator(packet *radius.Packet, secret []byte) bool {
	origMA, err := rfc2869.MessageAuthenticator_Lookup(packet)
	if err != nil {
		return false
	}
	if len(origMA) != 16 {
		return false
	}

	// 属性値を16バイトゼロに置換
	zeroMA := make([]byte, 16)
	_ = rfc2869.MessageAuthenticator_Set(packet, zeroMA)

	// パケットをバイト列化
	data, err := packet.MarshalBinary()
	if err != nil {
		_ = rfc2869.MessageAuthenticator_Set(packet, origMA)
		return false
	}

	// HMAC-MD5を計算
	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	expected := mac.Sum(nil)

	// 元の値を復元
	_ = rfc2869.MessageAuthenticator_Set(packet, origMA)

	return hmac.Equal(expected, origMA)
}

// SetMessageAuthenticator は応答パケットにMessage-Authenticator属性を生成・設定する。
func SetMessageAuthenticator(packet *radius.Packet, secret []byte, requestAuth [16]byte) {
	// 16バイトゼロをプレースホルダーとして設定
	zeroMA := make([]byte, 16)
	_ = rfc2869.MessageAuthenticator_Set(packet, zeroMA)

	// Request Authenticatorを使用
	savedAuth := packet.Authenticator
	packet.Authenticator = requestAuth

	data, err := packet.MarshalBinary()
	if err != nil {
		packet.Authenticator = savedAuth
		return
	}

	// HMAC-MD5を計算
	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	computed := mac.Sum(nil)

	// Authenticatorを復元
	packet.Authenticator = savedAuth

	// 計算結果で上書き
	_ = rfc2869.MessageAuthenticator_Set(packet, computed)
}
