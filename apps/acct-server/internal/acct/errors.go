package acct

import "errors"

var (
	// ErrUnknownStatusType は未知のAcct-Status-Typeの場合のエラー
	ErrUnknownStatusType = errors.New("unknown Acct-Status-Type")
)

// SequenceError は順序異常エラー
type SequenceError struct {
	Reason string
}

func (e *SequenceError) Error() string {
	return "sequence error: " + e.Reason
}
