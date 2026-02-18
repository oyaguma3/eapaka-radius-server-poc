package policy

import (
	"errors"
	"fmt"

	eapaka "github.com/oyaguma3/go-eapaka"
)

// GenerateMPPEKeys はMSKからMS-MPPE-Send/Recv-Key AVP値を生成する。
// MSKの先頭32バイトをRecv-Key、後半32バイトをSend-Keyとして暗号化する（RFC 2548準拠）。
func GenerateMPPEKeys(msk, secret, reqAuth []byte) (recvKey, sendKey []byte, err error) {
	if len(msk) < 64 {
		return nil, nil, errors.New("MSK must be at least 64 bytes")
	}

	recvKeyPlain := msk[:32]
	sendKeyPlain := msk[32:64]

	recvKey, err = eapaka.EncryptMPPEKey(recvKeyPlain, secret, reqAuth)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt recv key: %w", err)
	}

	sendKey, err = eapaka.EncryptMPPEKey(sendKeyPlain, secret, reqAuth)
	if err != nil {
		return nil, nil, fmt.Errorf("encrypt send key: %w", err)
	}

	return recvKey, sendKey, nil
}

// VlanAVPs はVLAN割り当て用のRADIUS AVPを生成する。
// Tunnel-Type=13(VLAN), Tunnel-Medium-Type=6(802), Tunnel-Private-Group-Id=vlanID。
// vlanIDが空文字の場合はnilを返す。
func VlanAVPs(vlanID string) map[string]any {
	if vlanID == "" {
		return nil
	}
	return map[string]any{
		"Tunnel-Type":             13,
		"Tunnel-Medium-Type":      6,
		"Tunnel-Private-Group-Id": vlanID,
	}
}

// SessionTimeoutValue はセッションタイムアウト値を検証し返却する。
// 0以下の場合はfalseを返す（設定なし）。
func SessionTimeoutValue(timeout int) (uint32, bool) {
	if timeout <= 0 {
		return 0, false
	}
	return uint32(timeout), true
}
