package radius

import (
	"log/slog"

	"layeh.com/radius"
)

// HandleStatusServer はStatus-Server(Code=12)を処理し、Accounting-Response(Code=5)を返す。
// D-10 4.7: RFC 5997準拠のヘルスチェック応答。
// Message-Authenticator検証失敗時はnilを返す（応答なし）。
func HandleStatusServer(request *radius.Packet, secret []byte, srcIP, traceID string) *radius.Packet {
	// 1. Message-Authenticator検証
	if !VerifyMessageAuthenticator(request, secret) {
		slog.Warn("Status-Server: Message-Authenticator検証失敗",
			"event_id", "RADIUS_AUTH_ERR",
			"trace_id", traceID,
			"src_ip", srcIP,
		)
		return nil
	}

	// 2. Accounting-Response応答を作成
	resp := request.Response(radius.CodeAccountingResponse)

	// 3. Proxy-Stateコピー
	proxyStates := extractProxyStatesRaw(request)
	ApplyProxyStates(resp, proxyStates)

	// 4. Message-Authenticator生成
	SetMessageAuthenticator(resp, secret, request.Authenticator)

	// Response Authenticatorはgo-radiusライブラリのEncode()が自動計算する

	slog.Info("Status-Server: 応答送信",
		"event_id", "PKT_RECV",
		"trace_id", traceID,
		"src_ip", srcIP,
	)

	return resp
}
