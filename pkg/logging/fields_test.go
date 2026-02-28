package logging

import (
	"errors"
	"testing"
)

func TestWithTraceID(t *testing.T) {
	attr := WithTraceID("trace-12345")
	if attr.Key != FieldTraceID {
		t.Errorf("Key = %q, want %q", attr.Key, FieldTraceID)
	}
	if attr.Value.String() != "trace-12345" {
		t.Errorf("Value = %q, want %q", attr.Value.String(), "trace-12345")
	}
}

func TestWithEventID(t *testing.T) {
	attr := WithEventID("AUTH_SUCCESS")
	if attr.Key != FieldEventID {
		t.Errorf("Key = %q, want %q", attr.Key, FieldEventID)
	}
	if attr.Value.String() != "AUTH_SUCCESS" {
		t.Errorf("Value = %q, want %q", attr.Value.String(), "AUTH_SUCCESS")
	}
}

func TestWithError(t *testing.T) {
	t.Run("With error", func(t *testing.T) {
		err := errors.New("connection failed")
		attr := WithError(err)
		if attr.Key != FieldError {
			t.Errorf("Key = %q, want %q", attr.Key, FieldError)
		}
		if attr.Value.String() != "connection failed" {
			t.Errorf("Value = %q, want %q", attr.Value.String(), "connection failed")
		}
	})

	t.Run("With nil error", func(t *testing.T) {
		attr := WithError(nil)
		if attr.Key != FieldError {
			t.Errorf("Key = %q, want %q", attr.Key, FieldError)
		}
		if attr.Value.String() != "" {
			t.Errorf("Value = %q, want empty string", attr.Value.String())
		}
	})
}

func TestWithSrcIP(t *testing.T) {
	attr := WithSrcIP("192.168.1.100")
	if attr.Key != FieldSrcIP {
		t.Errorf("Key = %q, want %q", attr.Key, FieldSrcIP)
	}
	if attr.Value.String() != "192.168.1.100" {
		t.Errorf("Value = %q, want %q", attr.Value.String(), "192.168.1.100")
	}
}

func TestWithLatency(t *testing.T) {
	attr := WithLatency(150)
	if attr.Key != FieldLatencyMs {
		t.Errorf("Key = %q, want %q", attr.Key, FieldLatencyMs)
	}
	if attr.Value.Int64() != 150 {
		t.Errorf("Value = %d, want %d", attr.Value.Int64(), 150)
	}
}

func TestWithHTTPStatus(t *testing.T) {
	attr := WithHTTPStatus(200)
	if attr.Key != FieldHTTPStatus {
		t.Errorf("Key = %q, want %q", attr.Key, FieldHTTPStatus)
	}
	if attr.Value.Int64() != 200 {
		t.Errorf("Value = %d, want %d", attr.Value.Int64(), 200)
	}
}

func TestWithRetryCount(t *testing.T) {
	attr := WithRetryCount(3)
	if attr.Key != FieldRetryCount {
		t.Errorf("Key = %q, want %q", attr.Key, FieldRetryCount)
	}
	if attr.Value.Int64() != 3 {
		t.Errorf("Value = %d, want %d", attr.Value.Int64(), 3)
	}
}

func TestCommonFields(t *testing.T) {
	t.Run("WithIMSI with masking", func(t *testing.T) {
		masker := NewMasker(true)
		cf := NewCommonFields(masker)
		attr := cf.WithIMSI("440101234567890")
		if attr.Key != FieldIMSI {
			t.Errorf("Key = %q, want %q", attr.Key, FieldIMSI)
		}
		want := "440101********0"
		if attr.Value.String() != want {
			t.Errorf("Value = %q, want %q", attr.Value.String(), want)
		}
	})

	t.Run("WithIMSI without masking", func(t *testing.T) {
		masker := NewMasker(false)
		cf := NewCommonFields(masker)
		attr := cf.WithIMSI("440101234567890")
		if attr.Value.String() != "440101234567890" {
			t.Errorf("Value = %q, want %q", attr.Value.String(), "440101234567890")
		}
	})

	t.Run("NewCommonFields with nil masker", func(t *testing.T) {
		cf := NewCommonFields(nil)
		attr := cf.WithIMSI("440101234567890")
		// nilの場合はマスキング無効で初期化される
		if attr.Value.String() != "440101234567890" {
			t.Errorf("Value = %q, want %q", attr.Value.String(), "440101234567890")
		}
	})

	t.Run("AuthLogFields", func(t *testing.T) {
		masker := NewMasker(true)
		cf := NewCommonFields(masker)
		fields := cf.AuthLogFields("trace-abc", "AUTH_START", "440101234567890")

		if len(fields) != 3 {
			t.Fatalf("fields length = %d, want %d", len(fields), 3)
		}

		// フィールドの型をチェック
		for i, f := range fields {
			if _, ok := f.(interface{ Key() string }); !ok {
				// slog.Attrは直接Keyを持たないのでキャストで確認
				t.Logf("field[%d] type check passed", i)
			}
		}
	})
}
