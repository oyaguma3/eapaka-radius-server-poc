package eap

import "errors"

// Identity解析エラー
var (
	// ErrInvalidIdentity はIdentity文字列の形式が不正な場合のエラー
	ErrInvalidIdentity = errors.New("invalid identity format")

	// ErrUnsupportedIdentity はサポートされていないIdentity種別の場合のエラー
	ErrUnsupportedIdentity = errors.New("unsupported identity type")

	// ErrMissingRealm はIdentityにRealmが含まれていない場合のエラー
	ErrMissingRealm = errors.New("missing realm in identity")
)

// Challenge検証エラー
var (
	// ErrMACInvalid はAT_MACの検証に失敗した場合のエラー
	ErrMACInvalid = errors.New("AT_MAC verification failed")

	// ErrRESNotFound はAT_RESが見つからない場合のエラー
	ErrRESNotFound = errors.New("AT_RES not found")

	// ErrRESLengthMismatch はAT_RESの長さが不正な場合のエラー
	ErrRESLengthMismatch = errors.New("AT_RES length mismatch")

	// ErrRESMismatch はAT_RESの値が一致しない場合のエラー
	ErrRESMismatch = errors.New("AT_RES mismatch")
)

// AT_KDFエラー
var (
	// ErrKDFNotSupported はサポートされていないKDF値が指定された場合のエラー
	ErrKDFNotSupported = errors.New("unsupported KDF value")
)

// 再同期エラー
var (
	// ErrAUTSNotFound はAT_AUTSが見つからない場合のエラー
	ErrAUTSNotFound = errors.New("AT_AUTS not found")

	// ErrResyncLimitExceeded は再同期回数が上限を超えた場合のエラー
	ErrResyncLimitExceeded = errors.New("resync limit exceeded")
)

// 状態エラー
var (
	// ErrInvalidState はEAPステートマシンの状態が不正な場合のエラー
	ErrInvalidState = errors.New("invalid eap state")
)

// エンジンエラー
var (
	// ErrContextNotFound はEAPコンテキストが見つからない場合のエラー
	ErrContextNotFound = errors.New("eap context not found")

	// ErrEAPParseFailed はEAPパケットのパースに失敗した場合のエラー
	ErrEAPParseFailed = errors.New("eap packet parse failed")
)
