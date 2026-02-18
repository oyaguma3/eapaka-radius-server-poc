package radius

import (
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2868"
	"layeh.com/radius/vendors/microsoft"
)

// AcceptParams はAccess-Accept生成に必要なパラメータ
type AcceptParams struct {
	// EAPMessage はEAP-Successメッセージ
	EAPMessage []byte
	// MSK はMaster Session Key（64バイト以上）。
	// 先頭32バイトがRecv-Key、後半32バイトがSend-Keyとして使用される。
	// layeh.com/radiusのmicrosoft APIが平文キーを期待するため、MSKを直接渡す。
	MSK []byte
	// SessionID はセッションUUID（Class属性に格納）
	SessionID string
	// VlanID はVLAN ID（空文字なら設定しない）
	VlanID string
	// SessionTimeout はタイムアウト秒数（0以下なら設定しない）
	SessionTimeout int
	// ProxyStates はリクエストから抽出されたProxy-State属性
	ProxyStates *ProxyStates
}

// ChallengeParams はAccess-Challenge生成に必要なパラメータ
type ChallengeParams struct {
	// EAPMessage はEAP-Request/AKA-Challenge等のメッセージ
	EAPMessage []byte
	// State はState属性（Trace IDを格納）
	State []byte
	// ProxyStates はリクエストから抽出されたProxy-State属性
	ProxyStates *ProxyStates
}

// RejectParams はAccess-Reject生成に必要なパラメータ
type RejectParams struct {
	// EAPMessage はEAP-Failureメッセージ
	EAPMessage []byte
	// ProxyStates はリクエストから抽出されたProxy-State属性
	ProxyStates *ProxyStates
}

// BuildAccessAccept はAccess-Acceptパケットを構築する。
// D-09 5.7: 認証成功時の応答パケットを生成する。
func BuildAccessAccept(request *radius.Packet, secret []byte, params *AcceptParams) *radius.Packet {
	resp := request.Response(radius.CodeAccessAccept)

	// EAP-Message（EAP-Success）
	SetEAPMessage(resp, params.EAPMessage)

	// MS-MPPE-Recv-Key / Send-Key
	// microsoft APIは平文キーを受け取り、内部で暗号化する。
	// 暗号化にはresp.Secretとresp.Authenticatorを使用するため、
	// Request Authenticatorをセットしてから呼び出す。
	if len(params.MSK) >= 64 {
		resp.Authenticator = request.Authenticator
		_ = microsoft.MSMPPERecvKey_Set(resp, params.MSK[:32])
		_ = microsoft.MSMPPESendKey_Set(resp, params.MSK[32:64])
	}

	// Class属性（セッションUUID）
	if params.SessionID != "" {
		_ = rfc2865.Class_Set(resp, []byte(params.SessionID))
	}

	// Tunnel AVP（VLAN割り当て）
	// Tunnel-Type=13(VLAN), Tunnel-Medium-Type=6(IEEE 802), Tunnel-Private-Group-Id
	if params.VlanID != "" {
		// Tag=0: タグなし。VLAN(13)はrfc2868パッケージに定数未定義のため直接指定。
		_ = rfc2868.TunnelType_Set(resp, 0, 13)
		_ = rfc2868.TunnelMediumType_Set(resp, 0, rfc2868.TunnelMediumType_Value_IEEE802)
		_ = rfc2868.TunnelPrivateGroupID_SetString(resp, 0, params.VlanID)
	}

	// Session-Timeout
	if params.SessionTimeout > 0 {
		_ = rfc2865.SessionTimeout_Set(resp, rfc2865.SessionTimeout(params.SessionTimeout))
	}

	// Proxy-State
	params.ProxyStates.Apply(resp)

	// Message-Authenticator
	SetMessageAuthenticator(resp, secret, request.Authenticator)

	return resp
}

// BuildAccessChallenge はAccess-Challengeパケットを構築する。
// D-09 5.7: EAP認証継続時の応答パケットを生成する。
func BuildAccessChallenge(request *radius.Packet, secret []byte, params *ChallengeParams) *radius.Packet {
	resp := request.Response(radius.CodeAccessChallenge)

	// EAP-Message（EAP-Request/AKA-Challenge等）
	SetEAPMessage(resp, params.EAPMessage)

	// State属性（Trace ID）
	if len(params.State) > 0 {
		SetState(resp, params.State)
	}

	// Proxy-State
	params.ProxyStates.Apply(resp)

	// Message-Authenticator
	SetMessageAuthenticator(resp, secret, request.Authenticator)

	return resp
}

// BuildAccessReject はAccess-Rejectパケットを構築する。
// D-09 5.7: 認証失敗時の応答パケットを生成する。
func BuildAccessReject(request *radius.Packet, secret []byte, params *RejectParams) *radius.Packet {
	resp := request.Response(radius.CodeAccessReject)

	// EAP-Message（EAP-Failure）
	if len(params.EAPMessage) > 0 {
		SetEAPMessage(resp, params.EAPMessage)
	}

	// Proxy-State
	params.ProxyStates.Apply(resp)

	// Message-Authenticator
	SetMessageAuthenticator(resp, secret, request.Authenticator)

	return resp
}
