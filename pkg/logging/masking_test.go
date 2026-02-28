package logging

import "testing"

func TestMaskIMSI(t *testing.T) {
	tests := []struct {
		name    string
		imsi    string
		enabled bool
		want    string
	}{
		{
			name:    "Standard IMSI with masking enabled",
			imsi:    "440101234567890",
			enabled: true,
			want:    "440101********0",
		},
		{
			name:    "Standard IMSI with masking disabled",
			imsi:    "440101234567890",
			enabled: false,
			want:    "440101234567890",
		},
		{
			name:    "Short IMSI with masking enabled",
			imsi:    "12345",
			enabled: true,
			want:    "12345", // 7文字以下はマスキングなし
		},
		{
			name:    "Empty IMSI",
			imsi:    "",
			enabled: true,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskIMSI(tt.imsi, tt.enabled)
			if got != tt.want {
				t.Errorf("MaskIMSI(%q, %v) = %q, want %q", tt.imsi, tt.enabled, got, tt.want)
			}
		})
	}
}

func TestMaskPartial(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		keepPrefix int
		keepSuffix int
		maskChar   rune
		want       string
	}{
		{
			name:       "Standard masking",
			s:          "1234567890",
			keepPrefix: 3,
			keepSuffix: 2,
			maskChar:   '*',
			want:       "123*****90",
		},
		{
			name:       "Different mask character",
			s:          "abcdefghij",
			keepPrefix: 2,
			keepSuffix: 3,
			maskChar:   'X',
			want:       "abXXXXXhij",
		},
		{
			name:       "String too short",
			s:          "abc",
			keepPrefix: 2,
			keepSuffix: 2,
			maskChar:   '*',
			want:       "abc", // 文字列長 <= keepPrefix + keepSuffix
		},
		{
			name:       "Exact length",
			s:          "abcd",
			keepPrefix: 2,
			keepSuffix: 2,
			maskChar:   '*',
			want:       "abcd",
		},
		{
			name:       "One character to mask",
			s:          "abcde",
			keepPrefix: 2,
			keepSuffix: 2,
			maskChar:   '*',
			want:       "ab*de",
		},
		{
			name:       "Empty string",
			s:          "",
			keepPrefix: 2,
			keepSuffix: 2,
			maskChar:   '*',
			want:       "",
		},
		{
			name:       "Unicode string",
			s:          "あいうえおかきく",
			keepPrefix: 2,
			keepSuffix: 2,
			maskChar:   '＊',
			want:       "あい＊＊＊＊きく",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MaskPartial(tt.s, tt.keepPrefix, tt.keepSuffix, tt.maskChar)
			if got != tt.want {
				t.Errorf("MaskPartial(%q, %d, %d, %q) = %q, want %q",
					tt.s, tt.keepPrefix, tt.keepSuffix, string(tt.maskChar), got, tt.want)
			}
		})
	}
}

func TestMasker(t *testing.T) {
	t.Run("Masking enabled", func(t *testing.T) {
		m := NewMasker(true)
		if !m.IsEnabled() {
			t.Error("IsEnabled() = false, want true")
		}
		got := m.IMSI("440101234567890")
		want := "440101********0"
		if got != want {
			t.Errorf("IMSI() = %q, want %q", got, want)
		}
	})

	t.Run("Masking disabled", func(t *testing.T) {
		m := NewMasker(false)
		if m.IsEnabled() {
			t.Error("IsEnabled() = true, want false")
		}
		got := m.IMSI("440101234567890")
		want := "440101234567890"
		if got != want {
			t.Errorf("IMSI() = %q, want %q", got, want)
		}
	})
}
