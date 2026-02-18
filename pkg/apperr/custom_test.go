package apperr

import (
	"errors"
	"strings"
	"testing"
)

func TestValidationError(t *testing.T) {
	t.Run("Error message format", func(t *testing.T) {
		err := NewValidationError("imsi", "must be 15 digits")
		got := err.Error()
		if !strings.Contains(got, "validation error") {
			t.Errorf("error message should contain 'validation error': %s", got)
		}
		if !strings.Contains(got, "field=imsi") {
			t.Errorf("error message should contain 'field=imsi': %s", got)
		}
		if !strings.Contains(got, "message=must be 15 digits") {
			t.Errorf("error message should contain 'message=must be 15 digits': %s", got)
		}
	})

	t.Run("Fields are accessible", func(t *testing.T) {
		err := NewValidationError("ki", "invalid hex")
		if err.Field != "ki" {
			t.Errorf("Field = %q, want %q", err.Field, "ki")
		}
		if err.Message != "invalid hex" {
			t.Errorf("Message = %q, want %q", err.Message, "invalid hex")
		}
	})
}

func TestBackendError(t *testing.T) {
	t.Run("Error message without cause", func(t *testing.T) {
		err := NewBackendError("sim-backend", 500, nil)
		got := err.Error()
		if !strings.Contains(got, "backend error") {
			t.Errorf("error message should contain 'backend error': %s", got)
		}
		if !strings.Contains(got, "backendID=sim-backend") {
			t.Errorf("error message should contain 'backendID=sim-backend': %s", got)
		}
		if !strings.Contains(got, "statusCode=500") {
			t.Errorf("error message should contain 'statusCode=500': %s", got)
		}
	})

	t.Run("Error message with cause", func(t *testing.T) {
		cause := errors.New("connection refused")
		err := NewBackendError("hsm-backend", 502, cause)
		got := err.Error()
		if !strings.Contains(got, "cause=connection refused") {
			t.Errorf("error message should contain cause: %s", got)
		}
	})

	t.Run("Unwrap returns cause", func(t *testing.T) {
		cause := errors.New("timeout")
		err := NewBackendError("test", 504, cause)
		if err.Unwrap() != cause {
			t.Error("Unwrap should return the cause")
		}
	})

	t.Run("Unwrap returns nil when no cause", func(t *testing.T) {
		err := NewBackendError("test", 500, nil)
		if err.Unwrap() != nil {
			t.Error("Unwrap should return nil when no cause")
		}
	})

	t.Run("errors.Is with wrapped error", func(t *testing.T) {
		cause := ErrBackendCommunication
		err := NewBackendError("test", 502, cause)
		if !errors.Is(err, ErrBackendCommunication) {
			t.Error("errors.Is should find wrapped sentinel error")
		}
	})
}

func TestValkeyError(t *testing.T) {
	t.Run("Error message without cause", func(t *testing.T) {
		err := NewValkeyError("GET", "sub:440101234567890", nil)
		got := err.Error()
		if !strings.Contains(got, "valkey error") {
			t.Errorf("error message should contain 'valkey error': %s", got)
		}
		if !strings.Contains(got, "operation=GET") {
			t.Errorf("error message should contain 'operation=GET': %s", got)
		}
		if !strings.Contains(got, "key=sub:440101234567890") {
			t.Errorf("error message should contain 'key=sub:440101234567890': %s", got)
		}
	})

	t.Run("Error message with cause", func(t *testing.T) {
		cause := errors.New("WRONGTYPE Operation")
		err := NewValkeyError("HGET", "sess:abc", cause)
		got := err.Error()
		if !strings.Contains(got, "cause=WRONGTYPE Operation") {
			t.Errorf("error message should contain cause: %s", got)
		}
	})

	t.Run("Unwrap returns cause", func(t *testing.T) {
		cause := errors.New("connection lost")
		err := NewValkeyError("SET", "key", cause)
		if err.Unwrap() != cause {
			t.Error("Unwrap should return the cause")
		}
	})

	t.Run("errors.Is with wrapped sentinel error", func(t *testing.T) {
		cause := ErrValkeyConnection
		err := NewValkeyError("PING", "", cause)
		if !errors.Is(err, ErrValkeyConnection) {
			t.Error("errors.Is should find wrapped sentinel error")
		}
	})

	t.Run("Fields are accessible", func(t *testing.T) {
		err := NewValkeyError("DEL", "test:key", nil)
		if err.Operation != "DEL" {
			t.Errorf("Operation = %q, want %q", err.Operation, "DEL")
		}
		if err.Key != "test:key" {
			t.Errorf("Key = %q, want %q", err.Key, "test:key")
		}
	})
}

func TestEAPIdentityError(t *testing.T) {
	t.Run("Error message format", func(t *testing.T) {
		err := NewEAPIdentityError("0440101234567890@nai.epc.mnc001.mcc440.3gppnetwork.org",
			"permanent", "invalid MCC format")
		got := err.Error()
		if !strings.Contains(got, "EAP identity error") {
			t.Errorf("error message should contain 'EAP identity error': %s", got)
		}
		if !strings.Contains(got, "type=permanent") {
			t.Errorf("error message should contain 'type=permanent': %s", got)
		}
		if !strings.Contains(got, "reason=invalid MCC format") {
			t.Errorf("error message should contain 'reason=invalid MCC format': %s", got)
		}
	})

	t.Run("Fields are accessible", func(t *testing.T) {
		err := NewEAPIdentityError("test@realm", "pseudonym", "not found")
		if err.Identity != "test@realm" {
			t.Errorf("Identity = %q, want %q", err.Identity, "test@realm")
		}
		if err.IdentityType != "pseudonym" {
			t.Errorf("IdentityType = %q, want %q", err.IdentityType, "pseudonym")
		}
		if err.Reason != "not found" {
			t.Errorf("Reason = %q, want %q", err.Reason, "not found")
		}
	})
}
