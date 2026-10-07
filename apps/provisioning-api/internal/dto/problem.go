// Package dto は Provisioning API の要求・応答の JSON（camelCase）を定義する（D-13 §3、OpenAPI 定義）。
package dto

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// ContentTypeProblem はエラー応答の Content-Type。
const ContentTypeProblem = "application/problem+json"

// ProblemDetails の cause の値（D-13 §4.3。aka-only-server の管理API と同じ値に、本APIの値を加えたもの）。
const (
	CauseInvalidMsgFormat        = "INVALID_MSG_FORMAT"
	CauseInvalidQueryParam       = "INVALID_QUERY_PARAM"
	CauseMandatoryIEMissing      = "MANDATORY_IE_MISSING"
	CauseMandatoryIEIncorrect    = "MANDATORY_IE_INCORRECT"
	CauseOptionalIEIncorrect     = "OPTIONAL_IE_INCORRECT"
	CauseUserNotFound            = "USER_NOT_FOUND"
	CauseClientNotFound          = "CLIENT_NOT_FOUND"
	CausePolicyNotFound          = "POLICY_NOT_FOUND"
	CauseSubscriberAlreadyExists = "SUBSCRIBER_ALREADY_EXISTS"
	CauseClientAlreadyExists     = "CLIENT_ALREADY_EXISTS"
	CauseSystemFailure           = "SYSTEM_FAILURE"
)

// InvalidParam は不正だった項目と理由を表す。
type InvalidParam struct {
	Param  string `json:"param"`
	Reason string `json:"reason,omitempty"`
}

// ProblemDetails はエラー応答（RFC 7807）。`type` は出力しない（aka-only-server と同じ）。
// pkg/httputil.ProblemDetail（Vector API が使用）とは別の型とする（D-13 §4.3）。
type ProblemDetails struct {
	Title         string         `json:"title"`
	Status        int            `json:"status"`
	Detail        string         `json:"detail,omitempty"`
	Cause         string         `json:"cause,omitempty"`
	InvalidParams []InvalidParam `json:"invalidParams,omitempty"`
}

// NewProblem は ProblemDetails を生成する。title は HTTP ステータスの文言にする。
func NewProblem(status int, cause, detail string) *ProblemDetails {
	return &ProblemDetails{Title: http.StatusText(status), Status: status, Cause: cause, Detail: detail}
}

// Optional は JSON Merge Patch の1項目を表す。
// 項目が指定されたか（Set）と、null が指定されたか（Null）を区別する。
type Optional[T any] struct {
	Set   bool
	Null  bool
	Value T
}

// UnmarshalJSON は項目が指定されたことを記録して値を読む。
// 型の違う値はエラーにする（要求全体が INVALID_MSG_FORMAT になる。作成の要求と同じ扱い）。
func (o *Optional[T]) UnmarshalJSON(b []byte) error {
	o.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Null = true
		return nil
	}
	return json.Unmarshal(b, &o.Value)
}
