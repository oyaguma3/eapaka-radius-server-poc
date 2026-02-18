package radius

import (
	"crypto/md5"
	"crypto/subtle"

	"layeh.com/radius"
)

// VerifyAccountingAuthenticator はAccounting-RequestのRequest Authenticatorを検証する（RFC 2866）。
// 検証式: Authenticator = MD5(Code + ID + Length + 16 zero octets + Attributes + Secret)
func VerifyAccountingAuthenticator(packet *radius.Packet, secret []byte) bool {
	// パケットをバイト列化
	data, err := packet.MarshalBinary()
	if err != nil {
		return false
	}

	if len(data) < 20 {
		return false
	}

	// 元のAuthenticator（オフセット4-19）を保存
	var origAuth [16]byte
	copy(origAuth[:], data[4:20])

	// Authenticatorフィールドを16個のゼロバイトに置換
	copy(data[4:20], make([]byte, 16))

	// MD5(Code + ID + Length + 16 zero + Attributes + Secret)
	h := md5.New()
	h.Write(data)
	h.Write(secret)
	expected := h.Sum(nil)

	return subtle.ConstantTimeCompare(origAuth[:], expected) == 1
}
