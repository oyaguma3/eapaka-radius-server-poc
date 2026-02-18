package session

import "errors"

var (
	// ErrSessionNotFound はセッションが見つからない場合のエラー
	ErrSessionNotFound = errors.New("session not found")
)
