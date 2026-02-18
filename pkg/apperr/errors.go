// Package apperr は共通エラー定義を提供する。
package apperr

import "errors"

// 認証関連エラー
var (
	// ErrIMSINotFound はIMSIが見つからない場合のエラー
	ErrIMSINotFound = errors.New("IMSI not found")
	// ErrAuthFailed は認証失敗エラー
	ErrAuthFailed = errors.New("authentication failed")
	// ErrAuthResMismatch はRES不一致エラー
	ErrAuthResMismatch = errors.New("authentication response mismatch")
	// ErrAuthMACInvalid はMAC検証失敗エラー
	ErrAuthMACInvalid = errors.New("invalid MAC")
	// ErrAuthTimeout は認証タイムアウトエラー
	ErrAuthTimeout = errors.New("authentication timeout")
	// ErrAuthResyncLimit は再同期回数上限エラー
	ErrAuthResyncLimit = errors.New("resync limit exceeded")
	// ErrUnsupportedEAPType は未サポートのEAPタイプエラー
	ErrUnsupportedEAPType = errors.New("unsupported EAP type")
)

// セッション関連エラー
var (
	// ErrSessionNotFound はセッションが見つからない場合のエラー
	ErrSessionNotFound = errors.New("session not found")
	// ErrSessionExpired はセッション有効期限切れエラー
	ErrSessionExpired = errors.New("session expired")
	// ErrContextNotFound はEAPコンテキストが見つからない場合のエラー
	ErrContextNotFound = errors.New("EAP context not found")
)

// ポリシー関連エラー
var (
	// ErrPolicyNotFound はポリシーが見つからない場合のエラー
	ErrPolicyNotFound = errors.New("policy not found")
	// ErrPolicyDenied はポリシーによる拒否エラー
	ErrPolicyDenied = errors.New("policy denied")
)

// インフラ関連エラー
var (
	// ErrValkeyConnection はValkey接続エラー
	ErrValkeyConnection = errors.New("valkey connection error")
	// ErrValkeyCommand はValkeyコマンド実行エラー
	ErrValkeyCommand = errors.New("valkey command error")
	// ErrVectorAPI はVector Gateway APIエラー
	ErrVectorAPI = errors.New("vector API error")
)

// Vector Gateway関連エラー
var (
	// ErrBackendNotImplemented はバックエンド未実装エラー
	ErrBackendNotImplemented = errors.New("backend not implemented")
	// ErrBackendCommunication はバックエンド通信エラー
	ErrBackendCommunication = errors.New("backend communication error")
	// ErrInvalidRequest は不正なリクエストエラー
	ErrInvalidRequest = errors.New("invalid request")
)

// RADIUS関連エラー
var (
	// ErrClientNotFound はRADIUSクライアントが見つからない場合のエラー
	ErrClientNotFound = errors.New("RADIUS client not found")
	// ErrInvalidAuthenticator は不正なAuthenticatorエラー
	ErrInvalidAuthenticator = errors.New("invalid authenticator")
)

// バリデーション関連エラー
var (
	// ErrInvalidIMSI は不正なIMSI形式エラー
	ErrInvalidIMSI = errors.New("invalid IMSI format")
	// ErrInvalidHex は不正な16進数文字列エラー
	ErrInvalidHex = errors.New("invalid hex string")
)
