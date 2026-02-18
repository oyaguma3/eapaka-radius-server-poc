package vector

import (
	"errors"
	"fmt"
)

// センチネルエラー
var (
	// ErrCircuitOpen はCircuit BreakerがOpen状態の場合のエラー
	ErrCircuitOpen = errors.New("circuit breaker is open")

	// ErrInvalidResponse はVector Gatewayからのレスポンスが不正な場合のエラー
	ErrInvalidResponse = errors.New("invalid response from vector gateway")

	// ErrTraceIDMissing はコンテキストにTrace IDが設定されていない場合のエラー
	ErrTraceIDMissing = errors.New("trace id missing in context")
)

// APIError はHTTP APIエラーを表す
type APIError struct {
	StatusCode int
	Message    string
	Details    *ProblemDetails
}

func (e *APIError) Error() string {
	if e.Details != nil {
		return fmt.Sprintf("vector api error: %d %s - %s", e.StatusCode, e.Details.Title, e.Details.Detail)
	}
	return fmt.Sprintf("vector api error: %d %s", e.StatusCode, e.Message)
}

// IsNotFound はIMSI未登録エラーかどうかを判定する
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsBadRequest はリクエスト不正エラーかどうかを判定する
func (e *APIError) IsBadRequest() bool {
	return e.StatusCode == 400
}

// IsServerError はサーバーエラーかどうかを判定する
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= 500
}

// ConnectionError は接続エラーを表す
type ConnectionError struct {
	Cause error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection error: %v", e.Cause)
}

func (e *ConnectionError) Unwrap() error {
	return e.Cause
}
