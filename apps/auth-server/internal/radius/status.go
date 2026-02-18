package radius

import (
	"log/slog"

	"layeh.com/radius"
)

// HandleStatusServer はStatus-Server(Code=12)を処理し、Access-Accept応答を返す。
// D-09 5.8: RFC 5997準拠のヘルスチェック応答。
// Message-Authenticator検証失敗時はnilを返す（応答なし）。
func HandleStatusServer(request *radius.Packet, secret []byte, srcIP, traceID string) *radius.Packet {
	// 1. Message-Authenticator検証
	if !VerifyMessageAuthenticator(request, secret) {
		slog.Warn("Status-Server: Message-Authenticator検証失敗",
			"event_id", "RADIUS_STATUS_AUTH_FAIL",
			"trace_id", traceID,
			"src_ip", srcIP,
		)
		return nil
	}

	// 2. Access-Accept応答を作成（EAP-Messageなし）
	resp := request.Response(radius.CodeAccessAccept)

	// 3. Proxy-Stateコピー
	ExtractProxyStates(request).Apply(resp)

	// 4. Message-Authenticator生成
	SetMessageAuthenticator(resp, secret, request.Authenticator)

	slog.Info("Status-Server: 応答送信",
		"event_id", "RADIUS_STATUS_OK",
		"trace_id", traceID,
		"src_ip", srcIP,
	)

	return resp
}
