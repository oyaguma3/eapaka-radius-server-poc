// Package service は Provisioning API の業務処理を提供する（D-13 §7.1）。
// 入力の検証・正規化は Admin TUI と同じ pkg/validation で、Valkey の読み書きは pkg/masterdata で行い、
// 成功した変更操作と秘密の値の読み出しを監査ログに記録する。
package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/validation"
	"github.com/redis/go-redis/v9"
)

// 一覧のページングの既定値と上限（D-13 §4.1）
const (
	defaultLimit = 50
	maxLimit     = 500
)

var (
	imsiPrefixPattern = regexp.MustCompile(`^[0-9]{1,15}$`)
	cursorPattern     = regexp.MustCompile(`^[0-9]{1,15}$`)
)

// Service は加入者・RADIUSクライアント・認可ポリシーの操作を提供する。
type Service struct {
	subs     *masterdata.SubscriberStore
	clients  *masterdata.ClientStore
	policies *masterdata.PolicyStore
	audit    *audit.Logger
	now      func() time.Time
}

// New は新しい Service を生成する。
func New(rdb *redis.Client, auditLogger *audit.Logger) *Service {
	return &Service{
		subs:     masterdata.NewSubscriberStore(rdb),
		clients:  masterdata.NewClientStore(rdb),
		policies: masterdata.NewPolicyStore(rdb),
		audit:    auditLogger,
		now:      time.Now,
	}
}

// Counts は各キーの件数を表す（/status 用）。
type Counts struct {
	Subscribers int64
	Clients     int64
	Policies    int64
}

// Counts は加入者・RADIUSクライアント・認可ポリシーの件数を返す。
func (s *Service) Counts(ctx context.Context) (*Counts, error) {
	var c Counts
	var err error
	if c.Subscribers, err = s.subs.Count(ctx); err != nil {
		return nil, err
	}
	if c.Clients, err = s.clients.Count(ctx); err != nil {
		return nil, err
	}
	if c.Policies, err = s.policies.Count(ctx); err != nil {
		return nil, err
	}
	return &c, nil
}

// ---- 検証エラー ----

// ValidationError は要求の検証エラーを表す。HTTP では 400 になる。
type ValidationError struct {
	// Detail は項目によらない説明（空の JSON Merge Patch 等）
	Detail    string
	Query     []dto.InvalidParam // クエリパラメーターの値が不正
	Missing   []dto.InvalidParam // 必須項目がない
	Mandatory []dto.InvalidParam // 必須項目の値が不正（パスの IMSI / IP を含む）
	Optional  []dto.InvalidParam // 任意項目の値が不正
}

// Error は error インターフェースを満たす。
func (v *ValidationError) Error() string {
	return "validation failed: " + v.Cause()
}

// Cause は ProblemDetails の cause を返す。
// クエリパラメーター、必須項目の欠落、必須項目の不正、任意項目の不正の順に優先する（aka-only-server と同じ）。
// 項目がなく Detail だけのときは、必須項目の欠落（空の JSON Merge Patch）とする。
func (v *ValidationError) Cause() string {
	switch {
	case len(v.Query) > 0:
		return dto.CauseInvalidQueryParam
	case len(v.Missing) > 0:
		return dto.CauseMandatoryIEMissing
	case len(v.Mandatory) > 0:
		return dto.CauseMandatoryIEIncorrect
	case len(v.Optional) > 0:
		return dto.CauseOptionalIEIncorrect
	default:
		return dto.CauseMandatoryIEMissing
	}
}

// Params は不正だった項目を、Cause と同じ優先順で返す。
func (v *ValidationError) Params() []dto.InvalidParam {
	var all []dto.InvalidParam
	for _, ps := range [][]dto.InvalidParam{v.Query, v.Missing, v.Mandatory, v.Optional} {
		all = append(all, ps...)
	}
	return all
}

// orNil は検証エラーがあれば v を、なければ nil を返す。
func (v *ValidationError) orNil() error {
	if v.Detail == "" && len(v.Params()) == 0 {
		return nil
	}
	return v
}

// required は必須項目を記録する。present が false なら欠落、err があれば値の不正とする。
func (v *ValidationError) required(param string, present bool, err error) {
	switch {
	case !present:
		v.Missing = append(v.Missing, dto.InvalidParam{Param: param, Reason: "is required"})
	case err != nil:
		v.Mandatory = append(v.Mandatory, dto.InvalidParam{Param: param, Reason: reason(err)})
	}
}

// optional は任意項目の値の不正を記録する。
func (v *ValidationError) optional(param string, err error) {
	if err != nil {
		v.Optional = append(v.Optional, dto.InvalidParam{Param: param, Reason: reason(err)})
	}
}

// patchField は JSON Merge Patch の1項目を検証し、書き込む値を返す。項目が指定されていなければ nil を返す。
// normalized は pkg/validation で正規化した値。null と不正な値は任意項目の不正として記録する。
func (v *ValidationError) patchField(param string, o dto.Optional[string], normalized string, validate func(string) error) *string {
	if !o.Set {
		return nil
	}
	if o.Null {
		v.Optional = append(v.Optional, dto.InvalidParam{Param: param, Reason: "must not be null"})
		return nil
	}
	if err := validate(normalized); err != nil {
		v.Optional = append(v.Optional, dto.InvalidParam{Param: param, Reason: reason(err)})
		return nil
	}
	return &normalized
}

// emptyPatch は項目が1つもない JSON Merge Patch のエラーを返す。
func emptyPatch() error {
	return &ValidationError{Detail: "at least one field is required"}
}

// reason は pkg/validation のエラーから、項目名を除いた理由を取り出す。
func reason(err error) string {
	var se *validation.SubscriberValidationError
	var ce *validation.ClientValidationError
	var pe *validation.PolicyValidationError
	switch {
	case errors.As(err, &se):
		return se.Message
	case errors.As(err, &ce):
		return ce.Message
	case errors.As(err, &pe):
		return pe.Message
	default:
		return err.Error()
	}
}

// checkIMSI はパスの IMSI を検証する。
func checkIMSI(imsi string) error {
	if err := validation.ValidateIMSI(imsi); err != nil {
		return &ValidationError{Mandatory: []dto.InvalidParam{{Param: "imsi", Reason: reason(err)}}}
	}
	return nil
}

// parseListQuery は一覧のクエリパラメーターを検証する。
func parseListQuery(q dto.ListQuery) (prefix, cursor string, limit int, err error) {
	var v ValidationError
	if q.Prefix != "" && !imsiPrefixPattern.MatchString(q.Prefix) {
		v.Query = append(v.Query, dto.InvalidParam{Param: "prefix", Reason: "must be 1 to 15 digits"})
	}
	if q.Cursor != "" && !cursorPattern.MatchString(q.Cursor) {
		v.Query = append(v.Query, dto.InvalidParam{Param: "cursor", Reason: "must be a nextCursor value from a previous response"})
	}
	limit = defaultLimit
	if q.Limit != "" {
		n, convErr := strconv.Atoi(q.Limit)
		if convErr != nil || n < 1 || n > maxLimit {
			v.Query = append(v.Query, dto.InvalidParam{Param: "limit", Reason: fmt.Sprintf("must be an integer between 1 and %d", maxLimit)})
		}
		limit = n
	}
	if err := v.orNil(); err != nil {
		return "", "", 0, err
	}
	return q.Prefix, q.Cursor, limit, nil
}

// ---- 監査ログの details ----

// changes は監査ログの details に記録する変更内容を組み立てる。
type changes []string

// value は値を記録してよい項目の変更前後を記録する。after が nil（指定なし）なら記録しない。
func (c *changes) value(name, before string, after *string) {
	if after != nil {
		*c = append(*c, fmt.Sprintf("%s: %s -> %s", name, before, *after))
	}
}

// secret は秘密の値（Ki / OPc、共有シークレット）の変更の有無だけを記録する。値は記録しない。
func (c *changes) secret(name, before string, after *string) {
	if after == nil {
		return
	}
	if before == *after {
		*c = append(*c, name+": unchanged")
	} else {
		*c = append(*c, name+": changed")
	}
}

func (c changes) String() string {
	return strings.Join(c, ", ")
}

// deref はポインターの値を返す。nil なら def を返す。
func deref(p *string, def string) string {
	if p == nil {
		return def
	}
	return *p
}
