// Package httputil はHTTP関連のユーティリティを提供する。
package httputil

import (
	"encoding/json"
	"net/http"
)

// ProblemDetail はRFC 7807準拠のエラーレスポンス構造体。
type ProblemDetail struct {
	Type   string `json:"type"`             // エラータイプのURI
	Title  string `json:"title"`            // エラータイトル
	Status int    `json:"status"`           // HTTPステータスコード
	Detail string `json:"detail,omitempty"` // 詳細説明
}

// NewProblemDetail は新しいProblemDetailを生成する。
func NewProblemDetail(status int, title, detail string) *ProblemDetail {
	return &ProblemDetail{
		Type:   "about:blank",
		Title:  title,
		Status: status,
		Detail: detail,
	}
}

// BadRequest は400 Bad Requestのエラーレスポンスを生成する。
func BadRequest(detail string) *ProblemDetail {
	return NewProblemDetail(http.StatusBadRequest, "Bad Request", detail)
}

// NotFound は404 Not Foundのエラーレスポンスを生成する。
func NotFound(detail string) *ProblemDetail {
	return NewProblemDetail(http.StatusNotFound, "Not Found", detail)
}

// InternalServerError は500 Internal Server Errorのエラーレスポンスを生成する。
func InternalServerError(detail string) *ProblemDetail {
	return NewProblemDetail(http.StatusInternalServerError, "Internal Server Error", detail)
}

// BadGateway は502 Bad Gatewayのエラーレスポンスを生成する。
func BadGateway(detail string) *ProblemDetail {
	return NewProblemDetail(http.StatusBadGateway, "Bad Gateway", detail)
}

// NotImplemented は501 Not Implementedのエラーレスポンスを生成する。
func NotImplemented(detail string) *ProblemDetail {
	return NewProblemDetail(http.StatusNotImplemented, "Not Implemented", detail)
}

// ServiceUnavailable は503 Service Unavailableのエラーレスポンスを生成する。
func ServiceUnavailable(detail string) *ProblemDetail {
	return NewProblemDetail(http.StatusServiceUnavailable, "Service Unavailable", detail)
}

// JSON はProblemDetailをJSON形式にエンコードする。
func (p *ProblemDetail) JSON() ([]byte, error) {
	return json.Marshal(p)
}

// MustJSON はProblemDetailをJSON形式にエンコードする。
// エンコードに失敗した場合はパニックする。
func (p *ProblemDetail) MustJSON() []byte {
	data, err := p.JSON()
	if err != nil {
		panic(err)
	}
	return data
}

// ContentType はRFC 7807で定義されたContent-Typeヘッダー値。
const ContentType = "application/problem+json"
