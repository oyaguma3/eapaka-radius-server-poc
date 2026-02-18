package logging

import "testing"

func TestMaskIMSI(t *testing.T) {
	tests := []struct {
		name    string
		imsi    string
		enabled bool
		want    string
	}{
		{"mask enabled, 15 digits", "440101234567890", true, "440101********0"},
		{"mask enabled, 10 digits", "4401012345", true, "440101********5"},
		{"mask enabled, 7 digits", "4401012", true, "440101********2"},
		{"mask enabled, 6 digits (boundary)", "440101", true, "440101"},
		{"mask enabled, 5 digits", "44010", true, "44010"},
		{"mask enabled, empty", "", true, ""},
		{"mask disabled, 15 digits", "440101234567890", false, "440101234567890"},
		{"mask disabled, empty", "", false, ""},
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
