package radius

import "errors"

// EAP-Message属性エラー
var (
	// ErrMissingEAPMessage はEAP-Message属性が見つからない場合のエラー
	ErrMissingEAPMessage = errors.New("EAP-Message attribute not found")
)

// Message-Authenticator属性エラー
var (
	// ErrMissingMessageAuthenticator はMessage-Authenticator属性が見つからない場合のエラー
	ErrMissingMessageAuthenticator = errors.New("message authenticator not found")

	// ErrInvalidMessageAuthenticator はMessage-Authenticator属性の検証に失敗した場合のエラー
	ErrInvalidMessageAuthenticator = errors.New("message authenticator verification failed")
)
