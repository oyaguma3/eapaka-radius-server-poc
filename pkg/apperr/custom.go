package apperr

import "fmt"

// ValidationError はバリデーションエラーを表す。
type ValidationError struct {
	Field   string // エラーが発生したフィールド名
	Message string // エラーメッセージ
}

// Error はerrorインターフェースを実装する。
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: field=%s, message=%s", e.Field, e.Message)
}

// NewValidationError はValidationErrorを生成する。
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}

// BackendError はバックエンドとの通信エラーを表す。
type BackendError struct {
	BackendID  string // バックエンドの識別子
	StatusCode int    // HTTPステータスコード
	Cause      error  // 根本原因
}

// Error はerrorインターフェースを実装する。
func (e *BackendError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("backend error: backendID=%s, statusCode=%d, cause=%v",
			e.BackendID, e.StatusCode, e.Cause)
	}
	return fmt.Sprintf("backend error: backendID=%s, statusCode=%d",
		e.BackendID, e.StatusCode)
}

// Unwrap は根本原因を返す。
func (e *BackendError) Unwrap() error {
	return e.Cause
}

// NewBackendError はBackendErrorを生成する。
func NewBackendError(backendID string, statusCode int, cause error) *BackendError {
	return &BackendError{
		BackendID:  backendID,
		StatusCode: statusCode,
		Cause:      cause,
	}
}

// ValkeyError はValkeyとの操作エラーを表す。
type ValkeyError struct {
	Operation string // 操作名（GET, SET, DEL等）
	Key       string // 操作対象のキー
	Cause     error  // 根本原因
}

// Error はerrorインターフェースを実装する。
func (e *ValkeyError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("valkey error: operation=%s, key=%s, cause=%v",
			e.Operation, e.Key, e.Cause)
	}
	return fmt.Sprintf("valkey error: operation=%s, key=%s", e.Operation, e.Key)
}

// Unwrap は根本原因を返す。
func (e *ValkeyError) Unwrap() error {
	return e.Cause
}

// NewValkeyError はValkeyErrorを生成する。
func NewValkeyError(operation, key string, cause error) *ValkeyError {
	return &ValkeyError{
		Operation: operation,
		Key:       key,
		Cause:     cause,
	}
}

// EAPIdentityError はEAP Identity関連のエラーを表す。
type EAPIdentityError struct {
	Identity     string // 受け取ったIdentity文字列
	IdentityType string // Identityの種類（permanent, pseudonym等）
	Reason       string // エラーの理由
}

// Error はerrorインターフェースを実装する。
func (e *EAPIdentityError) Error() string {
	return fmt.Sprintf("EAP identity error: identity=%s, type=%s, reason=%s",
		e.Identity, e.IdentityType, e.Reason)
}

// NewEAPIdentityError はEAPIdentityErrorを生成する。
func NewEAPIdentityError(identity, identityType, reason string) *EAPIdentityError {
	return &EAPIdentityError{
		Identity:     identity,
		IdentityType: identityType,
		Reason:       reason,
	}
}
