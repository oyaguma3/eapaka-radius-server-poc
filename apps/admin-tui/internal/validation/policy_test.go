package validation

import (
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/model"
)

func TestValidateDefaultAction(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"allow", false},
		{"deny", false},
		{"", true},
		{"ALLOW", true},
		{"other", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateDefaultAction(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDefaultAction(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateNasID(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"nas-01", false},
		{"*", false},
		{"nas.*", false},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateNasID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNasID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSSID(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"MySSID", false},
		{"SSID-01", false},
		{"", true},
		{"ThisIsAVeryLongSSIDThatExceeds32Ch", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateSSID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSSID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateAllowedSSIDs(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		wantErr bool
	}{
		{"single valid", []string{"ssid1"}, false},
		{"multiple valid", []string{"ssid1", "ssid2"}, false},
		{"empty list", []string{}, true},
		{"contains empty", []string{"ssid1", ""}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAllowedSSIDs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAllowedSSIDs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVlanID(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"", false}, // 空文字列はOK（未設定）
		{"0", false},
		{"100", false},
		{"4094", false},
		{"-1", true},
		{"4095", true},
		{"abc", true}, // 非数値はエラー
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateVlanID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVlanID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSessionTimeout(t *testing.T) {
	tests := []struct {
		input   int
		wantErr bool
	}{
		{0, false},
		{3600, false},
		{86400, false},
		{-1, true},
		{86401, true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			err := ValidateSessionTimeout(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSessionTimeout(%d) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePolicyRule(t *testing.T) {
	t.Run("valid rule", func(t *testing.T) {
		rule := &model.PolicyRule{
			NasID:          "*",
			AllowedSSIDs:   []string{"ssid1"},
			VlanID:         "100",
			SessionTimeout: 3600,
		}
		errs := ValidatePolicyRule(rule)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("invalid rule", func(t *testing.T) {
		rule := &model.PolicyRule{
			NasID:          "",
			AllowedSSIDs:   []string{},
			VlanID:         "-1",
			SessionTimeout: -1,
		}
		errs := ValidatePolicyRule(rule)
		if len(errs) != 4 {
			t.Errorf("expected 4 errors, got %d", len(errs))
		}
	})
}

func TestValidatePolicy(t *testing.T) {
	t.Run("valid policy", func(t *testing.T) {
		input := &PolicyInput{
			IMSI:    "440101234567890",
			Default: "deny",
			Rules: []model.PolicyRule{
				{
					NasID:        "*",
					AllowedSSIDs: []string{"ssid1"},
				},
			},
		}
		errs := ValidatePolicy(input)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("empty policy is valid", func(t *testing.T) {
		input := &PolicyInput{
			IMSI:    "440101234567890",
			Default: "deny",
			Rules:   []model.PolicyRule{},
		}
		errs := ValidatePolicy(input)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("invalid policy", func(t *testing.T) {
		input := &PolicyInput{
			IMSI:    "",
			Default: "invalid",
			Rules:   []model.PolicyRule{},
		}
		errs := ValidatePolicy(input)
		if len(errs) != 2 {
			t.Errorf("expected 2 errors, got %d", len(errs))
		}
	})
}

func TestNormalizePolicyInput(t *testing.T) {
	input := &PolicyInput{
		IMSI:    "  440101234567890  ",
		Default: "  ALLOW  ",
		Rules: []model.PolicyRule{
			{
				NasID:        "  *  ",
				AllowedSSIDs: []string{"  ssid1  ", "  ssid2  "},
				VlanID:       "100",
			},
		},
	}

	normalized := NormalizePolicyInput(input)

	if normalized.IMSI != "440101234567890" {
		t.Errorf("expected IMSI '440101234567890', got '%s'", normalized.IMSI)
	}
	if normalized.Default != "allow" {
		t.Errorf("expected Default 'allow', got '%s'", normalized.Default)
	}
	if normalized.Rules[0].NasID != "*" {
		t.Errorf("expected NasID '*', got '%s'", normalized.Rules[0].NasID)
	}
	if normalized.Rules[0].AllowedSSIDs[0] != "ssid1" {
		t.Errorf("expected AllowedSSIDs[0] 'ssid1', got '%s'", normalized.Rules[0].AllowedSSIDs[0])
	}
}
