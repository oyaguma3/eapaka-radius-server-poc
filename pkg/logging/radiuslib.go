package logging

import (
	"context"
	"log"
	"log/slog"
	"strings"
)

// EventRADIUSLibError は RADIUS ライブラリ（layeh.com/radius）が出すエラーの event_id。
const EventRADIUSLibError = "RADIUS_LIB_ERR"

// NewRADIUSLibraryLogger は layeh.com/radius の PacketServer.ErrorLog に設定する *log.Logger を返す。
// ライブラリは受信処理のエラー（パケットの解析失敗など）を標準の log パッケージで素のテキストとして出すため、
// JSON の slog（event_id: RADIUS_LIB_ERR）に流して、他のログと同じ形式で扱えるようにする。
//
// 共有シークレットが決まらずにパケットを捨てた場合（"empty secret returned from secret source"）は、
// アプリの SecretSource が送信元IP付きでログ（RADIUS_NO_SECRET 等）を出しているため、DEBUG にとどめる。
func NewRADIUSLibraryLogger() *log.Logger {
	return log.New(radiusLibWriter{}, "", 0)
}

// radiusLibWriter はライブラリが書き込んだ1行を slog に出力する。
type radiusLibWriter struct{}

func (radiusLibWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	level := slog.LevelWarn
	if strings.Contains(msg, "empty secret returned from secret source") {
		level = slog.LevelDebug
	}
	slog.Log(context.Background(), level, "RADIUSライブラリのエラー",
		FieldEventID, EventRADIUSLibError,
		FieldError, msg,
	)
	return len(p), nil
}
