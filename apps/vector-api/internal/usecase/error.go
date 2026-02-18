package usecase

import (
	"log/slog"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
)

// ProblemError はビジネスロジックエラーを表す。
type ProblemError struct {
	Status  int
	Title   string
	Detail  string
	Message string // ログメッセージ
	EventID string
}

// Error はerrorインターフェースを実装する。
func (e *ProblemError) Error() string {
	return e.Detail
}

// ToProblemDetail はProblemDetailに変換する。
func (e *ProblemError) ToProblemDetail() *dto.ProblemDetail {
	return dto.NewProblemDetail(e.Status, e.Title, e.Detail)
}

// LogLevel はログレベルを返す。
func (e *ProblemError) LogLevel() slog.Level {
	switch {
	case e.Status >= 500:
		return slog.LevelError
	case e.Status == 404:
		return slog.LevelInfo
	default:
		return slog.LevelWarn
	}
}

// 定義済みエラー
var (
	ErrSubscriberNotFound = &ProblemError{
		Status:  404,
		Title:   "User Not Found",
		Detail:  "IMSI does not exist in subscriber DB",
		Message: "subscriber not found",
		EventID: "CALC_ERR",
	}

	ErrInvalidIMSI = &ProblemError{
		Status:  400,
		Title:   "Bad Request",
		Detail:  "IMSI must be 15 digits",
		Message: "invalid IMSI format",
		EventID: "CALC_ERR",
	}

	ErrResyncMACFailed = &ProblemError{
		Status:  400,
		Title:   "Bad Request",
		Detail:  "AUTS MAC verification failed",
		Message: "AUTS MAC verification failed",
		EventID: "SQN_RESYNC_MAC_ERR",
	}

	ErrResyncInvalidFormat = &ProblemError{
		Status:  400,
		Title:   "Bad Request",
		Detail:  "Invalid AUTS format",
		Message: "invalid AUTS format",
		EventID: "SQN_RESYNC_FORMAT_ERR",
	}

	ErrResyncDeltaExceeded = &ProblemError{
		Status:  400,
		Title:   "Bad Request",
		Detail:  "SQN difference exceeds allowed range",
		Message: "SQN delta exceeded",
		EventID: "SQN_RESYNC_DELTA_ERR",
	}

	ErrSQNOverflow = &ProblemError{
		Status:  500,
		Title:   "Internal Server Error",
		Detail:  "Sequence number overflow",
		Message: "SQN overflow",
		EventID: "SQN_OVERFLOW_ERR",
	}

	ErrValkeyConnection = &ProblemError{
		Status:  500,
		Title:   "Internal Server Error",
		Detail:  "Database connection error",
		Message: "Valkey connection error",
		EventID: "VALKEY_CONN_ERR",
	}

	ErrMilenageCalculation = &ProblemError{
		Status:  500,
		Title:   "Internal Server Error",
		Detail:  "Authentication vector calculation failed",
		Message: "Milenage calculation error",
		EventID: "CALC_ERR",
	}
)
