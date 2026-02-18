package logging

import "log/slog"

// ログフィールド名の定数
const (
	FieldTraceID    = "trace_id"
	FieldEventID    = "event_id"
	FieldError      = "error"
	FieldSrcIP      = "src_ip"
	FieldLatencyMs  = "latency_ms"
	FieldHTTPStatus = "http_status"
	FieldRetryCount = "retry_count"
	FieldIMSI       = "imsi"
)

// WithTraceID はトレースIDのslog.Attrを返す。
func WithTraceID(traceID string) slog.Attr {
	return slog.String(FieldTraceID, traceID)
}

// WithEventID はイベントIDのslog.Attrを返す。
func WithEventID(eventID string) slog.Attr {
	return slog.String(FieldEventID, eventID)
}

// WithError はエラーのslog.Attrを返す。
func WithError(err error) slog.Attr {
	if err == nil {
		return slog.String(FieldError, "")
	}
	return slog.String(FieldError, err.Error())
}

// WithSrcIP はソースIPアドレスのslog.Attrを返す。
func WithSrcIP(ip string) slog.Attr {
	return slog.String(FieldSrcIP, ip)
}

// WithLatency はレイテンシ（ミリ秒）のslog.Attrを返す。
func WithLatency(ms int64) slog.Attr {
	return slog.Int64(FieldLatencyMs, ms)
}

// WithHTTPStatus はHTTPステータスコードのslog.Attrを返す。
func WithHTTPStatus(status int) slog.Attr {
	return slog.Int(FieldHTTPStatus, status)
}

// WithRetryCount はリトライ回数のslog.Attrを返す。
func WithRetryCount(count int) slog.Attr {
	return slog.Int(FieldRetryCount, count)
}

// CommonFields はマスキング設定を保持するログフィールド生成器。
type CommonFields struct {
	masker *Masker
}

// NewCommonFields は新しいCommonFieldsを生成する。
func NewCommonFields(masker *Masker) *CommonFields {
	if masker == nil {
		masker = NewMasker(false)
	}
	return &CommonFields{masker: masker}
}

// WithIMSI はマスキングされたIMSIのslog.Attrを返す。
func (cf *CommonFields) WithIMSI(imsi string) slog.Attr {
	return slog.String(FieldIMSI, cf.masker.IMSI(imsi))
}

// AuthLogFields は認証ログ用の共通フィールドを返す。
func (cf *CommonFields) AuthLogFields(traceID, eventID, imsi string) []any {
	return []any{
		WithTraceID(traceID),
		WithEventID(eventID),
		cf.WithIMSI(imsi),
	}
}
