package radius

import (
	"crypto/hmac"
	"crypto/md5"

	"layeh.com/radius"
	"layeh.com/radius/rfc2869"
)

// VerifyMessageAuthenticator はMessage-Authenticator属性を検証する。
// D-09 5.5/5.11: Request Authenticatorを使用してHMAC-MD5を計算し、
// パケット内のMessage-Authenticator値と比較する。
func VerifyMessageAuthenticator(packet *radius.Packet, secret []byte) bool {
	// 1. Message-Authenticator属性値を取得
	origMA, err := rfc2869.MessageAuthenticator_Lookup(packet)
	if err != nil {
		return false
	}
	if len(origMA) != 16 {
		return false
	}

	// 2. 属性値を16バイトゼロに置換
	zeroMA := make([]byte, 16)
	_ = rfc2869.MessageAuthenticator_Set(packet, zeroMA)

	// 3. パケットをバイト列化（MarshalBinaryはAuthenticatorをそのままコピー）
	data, err := packet.MarshalBinary()
	if err != nil {
		// 元の値を復元
		_ = rfc2869.MessageAuthenticator_Set(packet, origMA)
		return false
	}

	// 4. HMAC-MD5を計算
	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	expected := mac.Sum(nil)

	// 5. 元の値を復元
	_ = rfc2869.MessageAuthenticator_Set(packet, origMA)

	// 6. 比較
	return hmac.Equal(expected, origMA)
}

// SetMessageAuthenticator は応答パケットにMessage-Authenticator属性を生成・追加する。
// requestAuth はリクエストのAuthenticator（RFC 3579に基づき、Response計算時に使用）。
func SetMessageAuthenticator(packet *radius.Packet, secret []byte, requestAuth [16]byte) {
	// 1. Message-Authenticator属性に16バイトゼロをセット
	zeroMA := make([]byte, 16)
	_ = rfc2869.MessageAuthenticator_Set(packet, zeroMA)

	// 2. Request Authenticatorを使用（Response Authenticatorではない）
	savedAuth := packet.Authenticator
	packet.Authenticator = requestAuth

	// 3. パケットをバイト列化（MarshalBinaryはハッシュ計算なし）
	data, err := packet.MarshalBinary()
	if err != nil {
		// エラー時はAuthenticatorを復元して返す
		packet.Authenticator = savedAuth
		return
	}

	// 4. HMAC-MD5を計算
	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	computed := mac.Sum(nil)

	// 5. Authenticatorを復元
	packet.Authenticator = savedAuth

	// 6. 計算結果でMessage-Authenticator属性を上書き
	_ = rfc2869.MessageAuthenticator_Set(packet, computed)
}
