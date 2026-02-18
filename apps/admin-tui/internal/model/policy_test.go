package model

import (
	"testing"
)

func TestNewPolicy(t *testing.T) {
	p := NewPolicy("440101234567890", "deny")

	if p.IMSI != "440101234567890" {
		t.Errorf("expected IMSI to be '440101234567890', got '%s'", p.IMSI)
	}
	if p.Default != "deny" {
		t.Errorf("expected Default to be 'deny', got '%s'", p.Default)
	}
	if p.RulesJSON != "[]" {
		t.Errorf("expected RulesJSON to be '[]', got '%s'", p.RulesJSON)
	}
	if len(p.Rules) != 0 {
		t.Errorf("expected Rules to be empty, got %d rules", len(p.Rules))
	}
}

func TestPolicy_ParseRules(t *testing.T) {
	tests := []struct {
		name      string
		rulesJSON string
		wantLen   int
		wantErr   bool
	}{
		{
			name:      "empty JSON array",
			rulesJSON: "[]",
			wantLen:   0,
			wantErr:   false,
		},
		{
			name:      "empty string",
			rulesJSON: "",
			wantLen:   0,
			wantErr:   false,
		},
		{
			name:      "single rule",
			rulesJSON: `[{"nas_id":"*","allowed_ssids":["test-ssid"],"vlan_id":"100","session_timeout":3600}]`,
			wantLen:   1,
			wantErr:   false,
		},
		{
			name:      "multiple rules",
			rulesJSON: `[{"nas_id":"nas1","allowed_ssids":["ssid1"]},{"nas_id":"nas2","allowed_ssids":["ssid2"]}]`,
			wantLen:   2,
			wantErr:   false,
		},
		{
			name:      "invalid JSON",
			rulesJSON: `{invalid}`,
			wantLen:   0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &Policy{RulesJSON: tt.rulesJSON}
			err := p.ParseRules()

			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRules() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(p.Rules) != tt.wantLen {
				t.Errorf("ParseRules() got %d rules, want %d", len(p.Rules), tt.wantLen)
			}
		})
	}
}

func TestPolicy_EncodeRules(t *testing.T) {
	p := &Policy{
		Rules: []PolicyRule{
			{
				NasID:          "*",
				AllowedSSIDs:   []string{"test-ssid"},
				VlanID:         "100",
				SessionTimeout: 3600,
			},
		},
	}

	err := p.EncodeRules()
	if err != nil {
		t.Errorf("EncodeRules() error = %v", err)
		return
	}

	if p.RulesJSON == "" || p.RulesJSON == "[]" {
		t.Errorf("EncodeRules() got empty RulesJSON")
	}

	// Round-trip test
	p2 := &Policy{RulesJSON: p.RulesJSON}
	if err := p2.ParseRules(); err != nil {
		t.Errorf("ParseRules() after EncodeRules() error = %v", err)
		return
	}

	if len(p2.Rules) != 1 {
		t.Errorf("expected 1 rule after round-trip, got %d", len(p2.Rules))
		return
	}

	if p2.Rules[0].NasID != "*" {
		t.Errorf("expected NasID '*', got '%s'", p2.Rules[0].NasID)
	}
	if p2.Rules[0].VlanID != "100" {
		t.Errorf("expected VlanID '100', got '%s'", p2.Rules[0].VlanID)
	}
}

func TestPolicy_IsAllowByDefault(t *testing.T) {
	tests := []struct {
		defaultAction string
		want          bool
	}{
		{"allow", true},
		{"deny", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.defaultAction, func(t *testing.T) {
			p := &Policy{Default: tt.defaultAction}
			if got := p.IsAllowByDefault(); got != tt.want {
				t.Errorf("IsAllowByDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicy_Clone(t *testing.T) {
	original := &Policy{
		IMSI:      "440101234567890",
		Default:   "allow",
		RulesJSON: `[{"nas_id":"*","allowed_ssids":["ssid1","ssid2"]}]`,
		Rules: []PolicyRule{
			{
				NasID:        "*",
				AllowedSSIDs: []string{"ssid1", "ssid2"},
			},
		},
	}

	clone := original.Clone()

	// Modify clone
	clone.IMSI = "999999999999999"
	clone.Rules[0].AllowedSSIDs[0] = "modified"

	// Original should be unchanged
	if original.IMSI != "440101234567890" {
		t.Errorf("original IMSI was modified")
	}
	if original.Rules[0].AllowedSSIDs[0] != "ssid1" {
		t.Errorf("original AllowedSSIDs was modified")
	}
}
