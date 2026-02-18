package radius

import (
	"net"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2869"
)

// maxEAPMessageAttrLen はEAP-Message属性1つあたりの最大バイト数（RFC 3579）
const maxEAPMessageAttrLen = 253

// GetEAPMessage は全EAP-Message属性を受信順に結合して返す（RFC 3579）。
// EAP-Message属性が存在しない場合は(nil, false)を返す。
func GetEAPMessage(p *radius.Packet) ([]byte, bool) {
	eapMsg, err := rfc2869.EAPMessage_Lookup(p)
	if err != nil {
		return nil, false
	}
	if len(eapMsg) == 0 {
		return nil, false
	}
	return eapMsg, true
}

// SetEAPMessage はEAP-Message属性を設定する。
// 253バイト超のメッセージは自動的に分割される。
func SetEAPMessage(p *radius.Packet, eapMsg []byte) {
	_ = rfc2869.EAPMessage_Set(p, eapMsg)
}

// SplitEAPMessage はEAPメッセージを253バイト以下のチャンクに分割する。
func SplitEAPMessage(eapMsg []byte) [][]byte {
	if len(eapMsg) == 0 {
		return [][]byte{eapMsg}
	}

	var chunks [][]byte
	for len(eapMsg) > 0 {
		chunkSize := min(maxEAPMessageAttrLen, len(eapMsg))
		chunks = append(chunks, eapMsg[:chunkSize])
		eapMsg = eapMsg[chunkSize:]
	}
	return chunks
}

// GetState はState属性を取得する。
// 属性が存在しない場合は(nil, false)を返す。
func GetState(p *radius.Packet) ([]byte, bool) {
	state := rfc2865.State_Get(p)
	if state == nil {
		return nil, false
	}
	return state, true
}

// SetState はState属性を設定する。
func SetState(p *radius.Packet, state []byte) {
	_ = rfc2865.State_Set(p, state)
}

// GetNASIdentifier はNAS-Identifier属性を取得する。
// 属性が存在しない場合は("", false)を返す。
func GetNASIdentifier(p *radius.Packet) (string, bool) {
	val := rfc2865.NASIdentifier_GetString(p)
	if val == "" {
		return "", false
	}
	return val, true
}

// GetNASIPAddress はNAS-IP-Address属性を取得する。
// 属性が存在しない場合は(nil, false)を返す。
func GetNASIPAddress(p *radius.Packet) (net.IP, bool) {
	ip, err := rfc2865.NASIPAddress_Lookup(p)
	if err != nil {
		return nil, false
	}
	return ip, true
}

// GetCalledStationID はCalled-Station-Id属性を取得する。
// 属性が存在しない場合は("", false)を返す。
func GetCalledStationID(p *radius.Packet) (string, bool) {
	val := rfc2865.CalledStationID_GetString(p)
	if val == "" {
		return "", false
	}
	return val, true
}

// GetCallingStationID はCalling-Station-Id属性を取得する。
// 属性が存在しない場合は("", false)を返す。
func GetCallingStationID(p *radius.Packet) (string, bool) {
	val := rfc2865.CallingStationID_GetString(p)
	if val == "" {
		return "", false
	}
	return val, true
}

// GetUserName はUser-Name属性を取得する。
// 属性が存在しない場合は("", false)を返す。
func GetUserName(p *radius.Packet) (string, bool) {
	val := rfc2865.UserName_GetString(p)
	if val == "" {
		return "", false
	}
	return val, true
}
