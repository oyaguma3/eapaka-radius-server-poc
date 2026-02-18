package milenage

import (
	"encoding/hex"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
)

// HexDecode はHex文字列をバイト列に変換する。
func HexDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// HexEncode はバイト列をHex文字列に変換する。
func HexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

// VectorToResponse はVectorをVectorResponseに変換する。
func VectorToResponse(v *Vector) *dto.VectorResponse {
	return &dto.VectorResponse{
		RAND: HexEncode(v.RAND),
		AUTN: HexEncode(v.AUTN),
		XRES: HexEncode(v.XRES),
		CK:   HexEncode(v.CK),
		IK:   HexEncode(v.IK),
	}
}
