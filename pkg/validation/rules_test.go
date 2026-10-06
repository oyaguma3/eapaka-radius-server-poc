package validation

import "testing"

func TestIMSIPattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"440101234567890", true},
		{"123456789012345", true},
		{"12345678901234", false},   // 14 digits
		{"1234567890123456", false}, // 16 digits
		{"44010123456789a", false},  // contains letter
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IMSIPattern.MatchString(tt.input); got != tt.want {
				t.Errorf("IMSIPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestKiPattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"00112233445566778899AABBCCDDEEFF", true},
		{"00112233445566778899aabbccddeeff", true},
		{"0011223344556677889aabbccddeeff", false},   // 31 chars
		{"00112233445566778899aabbccddeefff", false}, // 33 chars
		{"00112233445566778899AABBCCDDEEGG", false},  // invalid hex
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := KiPattern.MatchString(tt.input); got != tt.want {
				t.Errorf("KiPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestOPcPattern(t *testing.T) {
	// OPc has same pattern as Ki
	tests := []struct {
		input string
		want  bool
	}{
		{"00112233445566778899AABBCCDDEEFF", true},
		{"00112233445566778899aabbccddeeff", true},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := OPcPattern.MatchString(tt.input); got != tt.want {
				t.Errorf("OPcPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAMFPattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"8000", true},
		{"FFFF", true},
		{"ffff", true},
		{"0000", true},
		{"800", false},   // 3 chars
		{"80000", false}, // 5 chars
		{"GGGG", false},  // invalid hex
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := AMFPattern.MatchString(tt.input); got != tt.want {
				t.Errorf("AMFPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSQNPattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"000000000000", true},
		{"FFFFFFFFFFFF", true},
		{"ffffffffffff", true},
		{"00000000000", false},   // 11 chars
		{"0000000000000", false}, // 13 chars
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := SQNPattern.MatchString(tt.input); got != tt.want {
				t.Errorf("SQNPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIPv4Pattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"192.168.1.1", true},
		{"0.0.0.0", true},
		{"255.255.255.255", true},
		{"10.0.0.1", true},
		{"256.1.1.1", false},
		{"192.168.1", false},
		{"192.168.1.1.1", false},
		{"192.168.1.a", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IPv4Pattern.MatchString(tt.input); got != tt.want {
				t.Errorf("IPv4Pattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSecretPattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"secret123", true},
		{"!@#$%^&*()", true},
		{"a", true},
		{" ", false}, // space not allowed
		{"secret with space", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := SecretPattern.MatchString(tt.input); got != tt.want {
				t.Errorf("SecretPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestClientNamePattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"ap-01", true},
		{"AP_01", true},
		{"client123", true},
		{"a", true},
		{"client with space", false},
		{"client@123", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := ClientNamePattern.MatchString(tt.input); got != tt.want {
				t.Errorf("ClientNamePattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestVendorPattern(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Cisco", true},
		{"HP-Aruba", true},
		{"Vendor Name", true},
		{"", true}, // empty is allowed
		{"Vendor@123", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := VendorPattern.MatchString(tt.input); got != tt.want {
				t.Errorf("VendorPattern.MatchString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
