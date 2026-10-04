// Package audit は監査ログ機能を提供する。
package audit

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

// Operation は監査ログの操作種別を表す。
type Operation string

const (
	// OpCreate は作成操作
	OpCreate Operation = "create"
	// OpUpdate は更新操作
	OpUpdate Operation = "update"
	// OpDelete は削除操作
	OpDelete Operation = "delete"
	// OpImport はインポート操作
	OpImport Operation = "import"
	// OpExport はエクスポート操作
	OpExport Operation = "export"
	// OpSearch は検索操作
	OpSearch Operation = "search"
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
	// TargetSession はセッション
	TargetSession TargetType = "session"
)

// Entry は監査ログエントリを表す。
type Entry struct {
	Time       string     `json:"time"`                  // RFC3339形式のタイムスタンプ
	Level      string     `json:"level"`                 // ログレベル（常に"INFO"）
	App        string     `json:"app"`                   // アプリケーション名（常に"admin-tui"）
	EventID    string     `json:"event_id"`              // イベントID（常に"AUDIT_LOG"）
	Msg        string     `json:"msg"`                   // メッセージ
	Operation  Operation  `json:"operation"`             // 操作種別
	TargetType TargetType `json:"target_type"`           // 対象種別
	TargetKey  string     `json:"target_key"`            // 対象キー
	TargetIMSI string     `json:"target_imsi,omitempty"` // 対象IMSI（該当時のみ）
	AdminUser  string     `json:"admin_user"`            // 管理者ユーザー
	Details    string     `json:"details,omitempty"`     // 追加詳細情報
	// RecordCount はインポート/エクスポートしたレコード件数（import / export のみ。0件も出力する）
	RecordCount *int `json:"record_count,omitempty"`
	// ResultCount は検索結果の件数（search のみ。検索に失敗した場合は出力しない）
	ResultCount *int `json:"result_count,omitempty"`
}

// Logger は監査ログを出力する。
type Logger struct {
	writer    io.Writer
	adminUser string
	mu        sync.Mutex
}

// NewLogger は新しいLoggerを生成する。
func NewLogger(adminUser string) *Logger {
	return &Logger{
		writer:    os.Stdout,
		adminUser: adminUser,
	}
}

// NewLoggerWithWriter は指定されたWriterを使用するLoggerを生成する。
func NewLoggerWithWriter(writer io.Writer, adminUser string) *Logger {
	return &Logger{
		writer:    writer,
		adminUser: adminUser,
	}
}

// Log は監査ログエントリを出力する。
func (l *Logger) Log(op Operation, targetType TargetType, targetKey, targetIMSI, msg string) {
	l.LogWithDetails(op, targetType, targetKey, targetIMSI, msg, "")
}

// LogWithDetails は詳細情報付きで監査ログエントリを出力する。
func (l *Logger) LogWithDetails(op Operation, targetType TargetType, targetKey, targetIMSI, msg, details string) {
	l.write(Entry{
		Operation:  op,
		TargetType: targetType,
		TargetKey:  targetKey,
		TargetIMSI: targetIMSI,
		Msg:        msg,
		Details:    details,
	})
}

// write は共通項目を設定して監査ログエントリを出力する。
func (l *Logger) write(e Entry) {
	entry := Entry{
		Time:        time.Now().UTC().Format(time.RFC3339),
		Level:       "INFO",
		App:         "admin-tui",
		EventID:     "AUDIT_LOG",
		Msg:         e.Msg,
		Operation:   e.Operation,
		TargetType:  e.TargetType,
		TargetKey:   e.TargetKey,
		TargetIMSI:  e.TargetIMSI,
		AdminUser:   l.adminUser,
		Details:     e.Details,
		RecordCount: e.RecordCount,
		ResultCount: e.ResultCount,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.writer.Write(append(data, '\n'))
}

// LogCreate はCREATE操作のログを出力する。
func (l *Logger) LogCreate(targetType TargetType, targetKey, targetIMSI string) {
	l.Log(OpCreate, targetType, targetKey, targetIMSI, string(targetType)+" created")
}

// LogUpdate はUPDATE操作のログを出力する。
func (l *Logger) LogUpdate(targetType TargetType, targetKey, targetIMSI string) {
	l.Log(OpUpdate, targetType, targetKey, targetIMSI, string(targetType)+" updated")
}

// LogDelete はDELETE操作のログを出力する。
func (l *Logger) LogDelete(targetType TargetType, targetKey, targetIMSI string) {
	l.Log(OpDelete, targetType, targetKey, targetIMSI, string(targetType)+" deleted")
}

// LogImport はIMPORT操作のログを出力する（件数を record_count に記録する）。
func (l *Logger) LogImport(targetType TargetType, count int, filename string) {
	l.write(Entry{
		Operation:   OpImport,
		TargetType:  targetType,
		TargetKey:   filename,
		Msg:         string(targetType) + " imported",
		RecordCount: &count,
	})
}

// LogExport はEXPORT操作のログを出力する（件数を record_count に記録する）。
func (l *Logger) LogExport(targetType TargetType, count int, filename string) {
	l.write(Entry{
		Operation:   OpExport,
		TargetType:  targetType,
		TargetKey:   filename,
		Msg:         string(targetType) + " exported",
		RecordCount: &count,
	})
}

// LogSearch はSEARCH操作のログを出力する。
// セッション検索（IMSIで検索）では検索したIMSIを target_imsi に、それ以外は検索語を details に記録する。
// 検索に成功した場合は結果件数を result_count に記録し、失敗した場合は details に理由を記録する。
func (l *Logger) LogSearch(targetType TargetType, query string, resultCount int, searchErr error) {
	e := Entry{
		Operation:  OpSearch,
		TargetType: targetType,
		Msg:        string(targetType) + " searched",
	}
	if targetType == TargetSession {
		e.TargetIMSI = query
	} else {
		e.Details = query
	}
	if searchErr != nil {
		failure := "search failed: " + searchErr.Error()
		if e.Details != "" {
			e.Details += "; " + failure
		} else {
			e.Details = failure
		}
	} else {
		e.ResultCount = &resultCount
	}
	l.write(e)
}
