package eap

import (
	"encoding/binary"
	"fmt"

	eapaka "github.com/oyaguma3/go-eapaka"
)

// EAP Type定数（RFC 3748）
const (
	EAPTypeIdentity uint8 = 1  // RFC 3748 Identity
	EAPTypeAKA      uint8 = 23 // RFC 4187 EAP-AKA
	EAPTypeAKAPrime uint8 = 50 // RFC 5448 EAP-AKA'
)

// GetEAPType はEAPパケットのType値を返す（バイト列から直接取得）
func GetEAPType(data []byte) uint8 {
	if len(data) < 5 {
		return 0
	}
	return data[4]
}

// GetEAPIdentifier はEAPパケットのIdentifier値を返す（バイト列から直接取得）
func GetEAPIdentifier(data []byte) uint8 {
	if len(data) < 2 {
		return 0
	}
	return data[1]
}

// ParseEAPPacket はバイト列をEAPパケットとしてパースする
func ParseEAPPacket(data []byte) (*eapaka.Packet, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("eap: empty packet data")
	}
	return eapaka.Parse(data)
}

// GetAttribute はパケットから指定型の属性を検索して返す
// 見つかった場合は(属性, true)、見つからない場合は(ゼロ値, false)を返す
func GetAttribute[T eapaka.Attribute](pkt *eapaka.Packet) (T, bool) {
	var zero T
	for _, attr := range pkt.Attributes {
		if v, ok := attr.(T); ok {
			return v, true
		}
	}
	return zero, false
}

// BuildEAPSuccess はEAP-Successパケットを構築する
func BuildEAPSuccess(identifier uint8) ([]byte, error) {
	// EAP-Success: Code(1) + Identifier(1) + Length(2) = 4バイト
	buf := make([]byte, 4)
	buf[0] = eapaka.CodeSuccess
	buf[1] = identifier
	binary.BigEndian.PutUint16(buf[2:4], 4)
	return buf, nil
}

// BuildEAPFailure はEAP-Failureパケットを構築する
func BuildEAPFailure(identifier uint8) ([]byte, error) {
	// EAP-Failure: Code(1) + Identifier(1) + Length(2) = 4バイト
	buf := make([]byte, 4)
	buf[0] = eapaka.CodeFailure
	buf[1] = identifier
	binary.BigEndian.PutUint16(buf[2:4], 4)
	return buf, nil
}

// BuildAKAIdentityRequest はフル認証誘導用のAKA-Identity Requestパケットを構築する
// AT_PERMANENT_ID_REQを含む
func BuildAKAIdentityRequest(identifier uint8, eapType uint8) ([]byte, error) {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeRequest,
		Identifier: identifier,
		Type:       eapType,
		Subtype:    eapaka.SubtypeIdentity,
		Attributes: []eapaka.Attribute{
			&eapaka.AtPermanentIdReq{},
		},
	}
	return pkt.Marshal()
}
