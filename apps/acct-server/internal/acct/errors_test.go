package acct

import (
	"strings"
	"testing"
)

func TestSequenceError_Error(t *testing.T) {
	tests := []struct {
		name   string
		reason string
		want   string
	}{
		{
			name:   "start_after_stop",
			reason: "start_after_stop",
			want:   "sequence error: start_after_stop",
		},
		{
			name:   "empty_reason",
			reason: "",
			want:   "sequence error: ",
		},
		{
			name:   "custom_reason",
			reason: "custom error reason",
			want:   "sequence error: custom error reason",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &SequenceError{Reason: tt.reason}
			got := err.Error()
			if got != tt.want {
				t.Errorf("SequenceError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSequenceError_ImplementsError(t *testing.T) {
	var err error = &SequenceError{Reason: "test"}

	if err == nil {
		t.Error("SequenceError should implement error interface")
	}

	if !strings.Contains(err.Error(), "sequence error") {
		t.Error("Error message should contain 'sequence error'")
	}
}

func TestErrUnknownStatusType(t *testing.T) {
	if ErrUnknownStatusType == nil {
		t.Error("ErrUnknownStatusType should not be nil")
	}

	expected := "unknown Acct-Status-Type"
	if ErrUnknownStatusType.Error() != expected {
		t.Errorf("ErrUnknownStatusType.Error() = %q, want %q", ErrUnknownStatusType.Error(), expected)
	}
}
