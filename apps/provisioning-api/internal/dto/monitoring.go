package dto

import (
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

// ---- 監査ログ（D-13 §3.6） ----

// AuditLogQuery は監査ログの一覧のクエリパラメーター（検証前の値）。
type AuditLogQuery struct {
	Before string
	Limit  string
}

// AuditLogEntry は監査ログの1件の応答。
// 項目は aka-only-server の管理API の監査ログ（id / time / operator / mgmtClient / action / target）にそろえ、
// 本APIの監査ログにある targetKey / traceId / details を加える。
type AuditLogEntry struct {
	ID   string    `json:"id"`
	Time time.Time `json:"time"`
	// Operator は X-Operator-Id の値（省略された操作では空文字）
	Operator   string `json:"operator"`
	MgmtClient string `json:"mgmtClient"`
	// Action は操作（例 subscriber.create、subscriber.keys.read、client.secret.read）
	Action string `json:"action"`
	// Target は対象（加入者・認可ポリシーは IMSI、RADIUSクライアントは ID）
	Target    string `json:"target"`
	TargetKey string `json:"targetKey"`
	TraceID   string `json:"traceId"`
	// Details は変更内容（秘密の値は含まない。ログファイルの details と同じ）
	Details string `json:"details,omitempty"`
}

// AuditLogList は監査ログの一覧の応答（新しい順）。
type AuditLogList struct {
	Items []AuditLogEntry `json:"items"`
	// NextBefore はさらに古いエントリがある場合だけ返す
	NextBefore string `json:"nextBefore,omitempty"`
}

// NewAuditLogEntry は応答用の監査ログを作る。
func NewAuditLogEntry(e audit.StoredEntry) AuditLogEntry {
	target := e.TargetIMSI
	if e.TargetType == audit.TargetClient {
		target = e.TargetID
	}
	if target == "" {
		target = e.TargetKey
	}
	return AuditLogEntry{
		ID:         e.ID,
		Time:       e.Time,
		Operator:   e.Operator,
		MgmtClient: e.MgmtClient,
		Action:     AuditAction(e.TargetType, e.Operation),
		Target:     target,
		TargetKey:  e.TargetKey,
		TraceID:    e.TraceID,
		Details:    e.Details,
	}
}

// AuditAction は監査ログの操作の名前を返す（aka-only-server と同じ命名）。
// 秘密の値の読み出しは、加入者では subscriber.keys.read、RADIUSクライアントでは client.secret.read とする。
func AuditAction(tt audit.TargetType, op audit.Operation) string {
	if op == audit.OpRead {
		switch tt {
		case audit.TargetSubscriber:
			return "subscriber.keys.read"
		case audit.TargetClient:
			return "client.secret.read"
		}
	}
	return string(tt) + "." + string(op)
}

// ---- セッション（D-13 §3.7） ----

// SessionQuery はセッションの一覧のクエリパラメーター（検証前の値）。
type SessionQuery struct {
	IMSI  string
	Limit string
}

// Session はセッションの応答（D-02 §3.E の sess:{UUID}）。
type Session struct {
	// ID はセッションの UUID（RADIUS の Class 属性の値）
	ID            string `json:"id"`
	IMSI          string `json:"imsi"`
	NasIP         string `json:"nasIp"`
	NasIdentifier string `json:"nasIdentifier"`
	// StartTime は接続開始日時（値がなければ省略）
	StartTime     string `json:"startTime,omitempty"`
	ClientIP      string `json:"clientIp"`
	AcctSessionID string `json:"acctSessionId"`
	InputOctets   int64  `json:"inputOctets"`
	OutputOctets  int64  `json:"outputOctets"`
}

// SessionList はセッションの一覧の応答（接続開始の新しい順）。
type SessionList struct {
	Items []Session `json:"items"`
	// Total は条件に一致するセッションの総数（items は limit 件まで）
	Total int64 `json:"total"`
}

// NewSession は応答用のセッションを作る。
func NewSession(s *model.Session) Session {
	out := Session{
		ID:            s.UUID,
		IMSI:          s.IMSI,
		NasIP:         s.NasIP,
		NasIdentifier: s.NasIdentifier,
		ClientIP:      s.ClientIP,
		AcctSessionID: s.AcctSessionID,
		InputOctets:   s.InputOctets,
		OutputOctets:  s.OutputOctets,
	}
	if s.StartTime > 0 {
		out.StartTime = time.Unix(s.StartTime, 0).UTC().Format(time.RFC3339)
	}
	return out
}
