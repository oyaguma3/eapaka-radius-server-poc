package apperr

import (
	"errors"
	"testing"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		// 認証関連
		{"ErrIMSINotFound", ErrIMSINotFound, "IMSI not found"},
		{"ErrAuthFailed", ErrAuthFailed, "authentication failed"},
		{"ErrAuthResMismatch", ErrAuthResMismatch, "authentication response mismatch"},
		{"ErrAuthMACInvalid", ErrAuthMACInvalid, "invalid MAC"},
		{"ErrAuthTimeout", ErrAuthTimeout, "authentication timeout"},
		{"ErrAuthResyncLimit", ErrAuthResyncLimit, "resync limit exceeded"},
		{"ErrUnsupportedEAPType", ErrUnsupportedEAPType, "unsupported EAP type"},
		// セッション関連
		{"ErrSessionNotFound", ErrSessionNotFound, "session not found"},
		{"ErrSessionExpired", ErrSessionExpired, "session expired"},
		{"ErrContextNotFound", ErrContextNotFound, "EAP context not found"},
		// ポリシー関連
		{"ErrPolicyNotFound", ErrPolicyNotFound, "policy not found"},
		{"ErrPolicyDenied", ErrPolicyDenied, "policy denied"},
		// インフラ関連
		{"ErrValkeyConnection", ErrValkeyConnection, "valkey connection error"},
		{"ErrValkeyCommand", ErrValkeyCommand, "valkey command error"},
		{"ErrVectorAPI", ErrVectorAPI, "vector API error"},
		// Vector Gateway関連
		{"ErrBackendNotImplemented", ErrBackendNotImplemented, "backend not implemented"},
		{"ErrBackendCommunication", ErrBackendCommunication, "backend communication error"},
		{"ErrInvalidRequest", ErrInvalidRequest, "invalid request"},
		// RADIUS関連
		{"ErrClientNotFound", ErrClientNotFound, "RADIUS client not found"},
		{"ErrInvalidAuthenticator", ErrInvalidAuthenticator, "invalid authenticator"},
		// バリデーション関連
		{"ErrInvalidIMSI", ErrInvalidIMSI, "invalid IMSI format"},
		{"ErrInvalidHex", ErrInvalidHex, "invalid hex string"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("%s.Error() = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	allErrors := []error{
		ErrIMSINotFound, ErrAuthFailed, ErrAuthResMismatch, ErrAuthMACInvalid,
		ErrAuthTimeout, ErrAuthResyncLimit, ErrUnsupportedEAPType,
		ErrSessionNotFound, ErrSessionExpired, ErrContextNotFound,
		ErrPolicyNotFound, ErrPolicyDenied,
		ErrValkeyConnection, ErrValkeyCommand, ErrVectorAPI,
		ErrBackendNotImplemented, ErrBackendCommunication, ErrInvalidRequest,
		ErrClientNotFound, ErrInvalidAuthenticator,
		ErrInvalidIMSI, ErrInvalidHex,
	}

	for i, err1 := range allErrors {
		for j, err2 := range allErrors {
			if i != j {
				if errors.Is(err1, err2) {
					t.Errorf("errors.Is(%v, %v) = true, want false", err1, err2)
				}
			}
		}
	}
}

func TestSentinelErrorsCanBeWrapped(t *testing.T) {
	wrapped := errors.New("wrapper: " + ErrIMSINotFound.Error())
	// 直接ラップではないのでIsはfalse
	if errors.Is(wrapped, ErrIMSINotFound) {
		t.Error("wrapped error should not match with errors.Is for simple wrapping")
	}

	// 正しいラップ方法
	wrappedCorrectly := errors.Join(errors.New("context"), ErrIMSINotFound)
	if !errors.Is(wrappedCorrectly, ErrIMSINotFound) {
		t.Error("correctly wrapped error should match with errors.Is")
	}
}
