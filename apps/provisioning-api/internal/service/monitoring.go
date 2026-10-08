package service

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/validation"
)

// 監査ログ・セッションの一覧の件数の既定値と上限（D-13 §4.1）
const (
	defaultAuditLimit   = 100
	maxAuditLimit       = 500
	defaultSessionLimit = 100
	maxSessionLimit     = 1000
)

// parseLimit は limit のクエリパラメーターを読む。空なら def、範囲外や数字でなければ検証エラーに記録する。
func parseLimit(v *ValidationError, raw string, def, max int) int {
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > max {
		v.Query = append(v.Query, dto.InvalidParam{Param: "limit", Reason: fmt.Sprintf("must be an integer between 1 and %d", max)})
	}
	return n
}

// AuditLogs は監査ログを新しい順に返す（GET /audit-logs）。
// さらに古いものがあれば、次に before に渡す値を next に入れる。
func (s *Service) AuditLogs(ctx context.Context, q dto.AuditLogQuery) (entries []audit.StoredEntry, next string, err error) {
	var v ValidationError
	if q.Before != "" && !audit.StreamIDPattern.MatchString(q.Before) {
		v.Query = append(v.Query, dto.InvalidParam{Param: "before", Reason: "must be an id or nextBefore value from a previous response"})
	}
	limit := parseLimit(&v, q.Limit, defaultAuditLimit, maxAuditLimit)
	if err := v.orNil(); err != nil {
		return nil, "", err
	}
	return s.auditStore.List(ctx, q.Before, limit)
}

// Sessions はアクティブセッションを接続開始の新しい順に返す（GET /sessions）。
// imsi を指定すると、その加入者のセッションだけを返す。total は条件に一致する件数（返すのは limit 件まで）。
// 読み出しだけで、索引（idx:user:{IMSI}）の掃除はしない（Admin TUI が行う）。
func (s *Service) Sessions(ctx context.Context, q dto.SessionQuery) (sessions []*model.Session, total int64, err error) {
	var v ValidationError
	if q.IMSI != "" {
		if err := validation.ValidateIMSI(q.IMSI); err != nil {
			v.Query = append(v.Query, dto.InvalidParam{Param: "imsi", Reason: reason(err)})
		}
	}
	limit := parseLimit(&v, q.Limit, defaultSessionLimit, maxSessionLimit)
	if err := v.orNil(); err != nil {
		return nil, 0, err
	}

	if q.IMSI != "" {
		sessions, _, err = s.sessions.ListByIMSI(ctx, q.IMSI)
	} else {
		sessions, err = s.sessions.List(ctx)
	}
	if err != nil {
		return nil, 0, err
	}
	slices.SortFunc(sessions, func(a, b *model.Session) int {
		return cmp.Or(cmp.Compare(b.StartTime, a.StartTime), cmp.Compare(a.UUID, b.UUID))
	})
	total = int64(len(sessions))
	return sessions[:min(len(sessions), limit)], total, nil
}
