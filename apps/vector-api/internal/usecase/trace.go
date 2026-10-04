package usecase

import "context"

// traceIDKey はコンテキストにTrace IDを格納するキー。
type traceIDKey struct{}

// ContextWithTraceID はTrace IDをコンテキストに設定する（ハンドラーから呼ぶ）。
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey{}, traceID)
}

// traceIDFromContext はコンテキストからTrace IDを取り出す。設定されていなければ空文字を返す。
func traceIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(traceIDKey{}).(string)
	return id
}
