package validation

import "testing"

func TestValidateIMSI(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"440101234567890", false},
		{"123456789012345", false},
		{"", true},
		{"12345678901234", true},
		{"1234567890123456", true},
		{"44010123456789a", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateIMSI(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIMSI(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateKi(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"00112233445566778899AABBCCDDEEFF", false},
		{"00112233445566778899aabbccddeeff", false},
		{"", true},
		{"00112233445566778899AABBCCDDEEF", true},   // 31 chars
		{"00112233445566778899AABBCCDDEEFFF", true}, // 33 chars
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateKi(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKi(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateOPc(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"00112233445566778899AABBCCDDEEFF", false},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateOPc(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOPc(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAMF(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"8000", false},
		{"FFFF", false},
		{"ffff", false},
		{"", true},
		{"800", true},
		{"80000", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateAMF(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAMF(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSQN(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"000000000000", false},
		{"FFFFFFFFFFFF", false},
		{"", true},
		{"00000000000", true},
		{"0000000000000", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateSQN(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSQN(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSubscriber(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		input := &SubscriberInput{
			IMSI: "440101234567890",
			Ki:   "00112233445566778899AABBCCDDEEFF",
			OPc:  "00112233445566778899AABBCCDDEEFF",
			AMF:  "8000",
			SQN:  "000000000000",
		}
		errs := ValidateSubscriber(input)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		input := &SubscriberInput{
			IMSI: "",
			Ki:   "",
			OPc:  "",
			AMF:  "",
			SQN:  "",
		}
		errs := ValidateSubscriber(input)
		if len(errs) != 5 {
			t.Errorf("expected 5 errors, got %d", len(errs))
		}
	})
}

func TestNormalizeSubscriberInput(t *testing.T) {
	input := &SubscriberInput{
		IMSI: "  440101234567890  ",
		Ki:   "  aabbccddeeff00112233445566778899  ",
		OPc:  "  00112233445566778899aabbccddeeff  ",
		AMF:  "  8000  ",
		SQN:  "  000000000000  ",
	}

	normalized := NormalizeSubscriberInput(input)

	if normalized.IMSI != "440101234567890" {
		t.Errorf("expected IMSI '440101234567890', got '%s'", normalized.IMSI)
	}
	if normalized.Ki != "AABBCCDDEEFF00112233445566778899" {
		t.Errorf("expected Ki 'AABBCCDDEEFF00112233445566778899', got '%s'", normalized.Ki)
	}
	if normalized.OPc != "00112233445566778899AABBCCDDEEFF" {
		t.Errorf("expected OPc '00112233445566778899AABBCCDDEEFF', got '%s'", normalized.OPc)
	}
	if normalized.AMF != "8000" {
		t.Errorf("expected AMF '8000', got '%s'", normalized.AMF)
	}
	if normalized.SQN != "000000000000" {
		t.Errorf("expected SQN '000000000000', got '%s'", normalized.SQN)
	}
}
