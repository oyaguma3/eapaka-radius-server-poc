package usecase

import (
	"log/slog"
	"testing"
)

func TestProblemError(t *testing.T) {
	t.Run("Error method", func(t *testing.T) {
		err := ErrSubscriberNotFound
		got := err.Error()
		if got != err.Detail {
			t.Errorf("Error() = %q, want %q", got, err.Detail)
		}
	})

	t.Run("ToProblemDetail", func(t *testing.T) {
		err := ErrInvalidIMSI
		pd := err.ToProblemDetail()

		if pd.Status != err.Status {
			t.Errorf("Status = %d, want %d", pd.Status, err.Status)
		}
		if pd.Title != err.Title {
			t.Errorf("Title = %q, want %q", pd.Title, err.Title)
		}
		if pd.Detail != err.Detail {
			t.Errorf("Detail = %q, want %q", pd.Detail, err.Detail)
		}
	})
}

func TestProblemErrorLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		err   *ProblemError
		level slog.Level
	}{
		{"500 error", ErrValkeyConnection, slog.LevelError},
		{"404 error", ErrSubscriberNotFound, slog.LevelInfo},
		{"400 error", ErrInvalidIMSI, slog.LevelWarn},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.LogLevel()
			if got != tt.level {
				t.Errorf("LogLevel() = %v, want %v", got, tt.level)
			}
		})
	}
}

func TestPredefinedErrors(t *testing.T) {
	errors := []*ProblemError{
		ErrSubscriberNotFound,
		ErrInvalidIMSI,
		ErrResyncMACFailed,
		ErrResyncInvalidFormat,
		ErrResyncDeltaExceeded,
		ErrSQNOverflow,
		ErrValkeyConnection,
		ErrMilenageCalculation,
	}

	for _, err := range errors {
		t.Run(err.EventID, func(t *testing.T) {
			if err.Status == 0 {
				t.Error("Status should not be 0")
			}
			if err.Title == "" {
				t.Error("Title should not be empty")
			}
			if err.Detail == "" {
				t.Error("Detail should not be empty")
			}
			if err.EventID == "" {
				t.Error("EventID should not be empty")
			}
		})
	}
}
