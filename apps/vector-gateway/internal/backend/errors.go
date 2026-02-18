package backend

import "fmt"

// ProblemDetail はRFC 7807準拠のエラーレスポンスを表す。
type ProblemDetail struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}

// NewProblemDetail は新しいProblemDetailを生成する。
func NewProblemDetail(status int, title, detail string) *ProblemDetail {
	return &ProblemDetail{
		Type:   "about:blank",
		Title:  title,
		Detail: detail,
		Status: status,
	}
}

// BackendNotImplementedError は未実装バックエンドへのアクセスエラーを表す。
type BackendNotImplementedError struct {
	ID string
}

func (e *BackendNotImplementedError) Error() string {
	return fmt.Sprintf("backend %q is not implemented", e.ID)
}

// BackendCommunicationError はバックエンドとの通信エラーを表す。
type BackendCommunicationError struct {
	Err error
}

func (e *BackendCommunicationError) Error() string {
	return fmt.Sprintf("backend communication error: %v", e.Err)
}

func (e *BackendCommunicationError) Unwrap() error {
	return e.Err
}

// BackendResponseError はバックエンドからのエラーレスポンスを表す。
// 4xxエラーをそのまま伝搬する場合に使用する。
type BackendResponseError struct {
	StatusCode int
	Problem    *ProblemDetail
}

func (e *BackendResponseError) Error() string {
	return fmt.Sprintf("backend returned status %d: %s", e.StatusCode, e.Problem.Detail)
}
