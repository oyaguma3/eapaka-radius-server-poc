package dto

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
