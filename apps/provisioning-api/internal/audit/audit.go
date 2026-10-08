// Package audit は監査ログを出力する（D-13 §6.2）。
// Admin TUI の監査ログ（D-04 §3.5）と同じ項目に、管理クライアントの識別名（mgmt_client）を加えて出力する。
package audit

import (
	"io"
	"log/slog"
)

// Operation は監査ログの操作種別を表す。
type Operation string

const (
	// OpCreate は作成
	OpCreate Operation = "create"
	// OpUpdate は変更
	OpUpdate Operation = "update"
	// OpDelete は削除
	OpDelete Operation = "delete"
	// OpRead は秘密の値（Ki / OPc、共有シークレット）の読み出し
	OpRead Operation = "read"
)

// TargetType は監査ログの対象種別を表す。
type TargetType string

const (
	// TargetSubscriber は加入者
	TargetSubscriber TargetType = "subscriber"
	// TargetClient はRADIUSクライアント
	TargetClient TargetType = "client"
	// TargetPolicy は認可ポリシー
	TargetPolicy TargetType = "policy"
)

// msgSuffix は操作種別ごとの msg の末尾（msg は「{target_type} {末尾}」。例: subscriber created）。
var msgSuffix = map[Operation]string{
	OpCreate: "created",
	OpUpdate: "updated",
	OpDelete: "deleted",
	OpRead:   "secret read",
}

// Actor は操作した者を表す。
type Actor struct {
	// Operator は X-Operator-Id の値（省略時は空文字）
	Operator string
	// MgmtClient はクライアント証明書に対応する管理クライアントの識別名
	MgmtClient string
	// TraceID はリクエストのトレースID
	TraceID string
}

// Entry は監査ログの1件を表す。
type Entry struct {
	Operation  Operation
	TargetType TargetType
	// TargetKey は Valkey のキー（sub:{IMSI} 等）
	TargetKey string
	// TargetID は RADIUSクライアントのID（RADIUSクライアントだけ。IP を変えても変わらない）
	TargetID string
	// TargetIMSI は加入者・認可ポリシーの IMSI（生値。RADIUSクライアントでは空文字）
	TargetIMSI string
	// Details は変更内容。秘密の値そのものは含めない
	Details string
}

// Logger は監査ログを出力する。
type Logger struct {
	log *slog.Logger
}

// NewLogger は w に JSON で出力する Logger を生成する。
// 監査ログは LOG_LEVEL によらず必ず出力するため、アプリケーションログとは別のロガーにする。
func NewLogger(w io.Writer) *Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})
	return &Logger{log: slog.New(h).With("app", "provisioning-api", "event_id", "AUDIT_LOG")}
}

// Record は監査ログを1件出力する。
func (l *Logger) Record(actor Actor, e Entry) {
	attrs := []any{
		"trace_id", actor.TraceID,
		"operation", string(e.Operation),
		"target_type", string(e.TargetType),
		"target_key", e.TargetKey,
	}
	if e.TargetID != "" {
		attrs = append(attrs, "target_id", e.TargetID)
	}
	if e.TargetIMSI != "" {
		attrs = append(attrs, "target_imsi", e.TargetIMSI)
	}
	attrs = append(attrs, "admin_user", actor.Operator, "mgmt_client", actor.MgmtClient)
	if e.Details != "" {
		attrs = append(attrs, "details", e.Details)
	}
	l.log.Info(string(e.TargetType)+" "+msgSuffix[e.Operation], attrs...)
}
