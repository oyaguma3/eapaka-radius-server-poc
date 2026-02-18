package session

import "errors"

// EAPコンテキスト関連エラー
var (
	// ErrContextNotFound はEAPコンテキストが見つからない場合のエラー
	ErrContextNotFound = errors.New("eap context not found")

	// ErrContextExpired はEAPコンテキストの有効期限が切れた場合のエラー
	ErrContextExpired = errors.New("eap context expired")

	// ErrContextInvalid はEAPコンテキストの内容が不正な場合のエラー
	ErrContextInvalid = errors.New("eap context invalid")
)

// セッション関連エラー
var (
	// ErrSessionNotFound はセッションが見つからない場合のエラー
	ErrSessionNotFound = errors.New("session not found")

	// ErrSessionExpired はセッションの有効期限が切れた場合のエラー
	ErrSessionExpired = errors.New("session expired")
)
