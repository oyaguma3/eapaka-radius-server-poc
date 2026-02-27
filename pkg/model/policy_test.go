package model

import "testing"

func TestNewPolicy(t *testing.T) {
	policy := NewPolicy("440101234567890", "allow")

	if policy.IMSI != "440101234567890" {
		t.Errorf("IMSI = %q, want %q", policy.IMSI, "440101234567890")
	}
	if policy.Default != "allow" {
		t.Errorf("Default = %q, want %q", policy.Default, "allow")
	}
	if policy.RulesJSON != "[]" {
		t.Errorf("RulesJSON = %q, want %q", policy.RulesJSON, "[]")
	}
	if len(policy.Rules) != 0 {
		t.Errorf("Rules length = %d, want %d", len(policy.Rules), 0)
	}
}

func TestPolicyParseRules(t *testing.T) {
	t.Run("Empty rules JSON", func(t *testing.T) {
		policy := NewPolicy("imsi", "allow")
		err := policy.ParseRules()
		if err != nil {
			t.Errorf("ParseRules() error = %v, want nil", err)
		}
		if len(policy.Rules) != 0 {
			t.Errorf("Rules length = %d, want %d", len(policy.Rules), 0)
		}
	})

	t.Run("Empty string", func(t *testing.T) {
		policy := &Policy{RulesJSON: ""}
		err := policy.ParseRules()
		if err != nil {
			t.Errorf("ParseRules() error = %v, want nil", err)
		}
		if len(policy.Rules) != 0 {
			t.Errorf("Rules length = %d, want %d", len(policy.Rules), 0)
		}
	})

	t.Run("Valid rules JSON", func(t *testing.T) {
		policy := &Policy{
			RulesJSON: `[{"nas_id":"AP-OFFICE-01","allowed_ssids":["CORP-WIFI","GUEST-WIFI"],"vlan_id":"100","session_timeout":3600}]`,
		}
		err := policy.ParseRules()
		if err != nil {
			t.Errorf("ParseRules() error = %v, want nil", err)
		}
		if len(policy.Rules) != 1 {
			t.Fatalf("Rules length = %d, want %d", len(policy.Rules), 1)
		}
		rule := policy.Rules[0]
		if rule.NasID != "AP-OFFICE-01" {
			t.Errorf("NasID = %q, want %q", rule.NasID, "AP-OFFICE-01")
		}
		if len(rule.AllowedSSIDs) != 2 {
			t.Errorf("AllowedSSIDs length = %d, want %d", len(rule.AllowedSSIDs), 2)
		}
		if rule.VlanID != "100" {
			t.Errorf("VlanID = %q, want %q", rule.VlanID, "100")
		}
		if rule.SessionTimeout != 3600 {
			t.Errorf("SessionTimeout = %d, want %d", rule.SessionTimeout, 3600)
		}
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		policy := &Policy{RulesJSON: `invalid json`}
		err := policy.ParseRules()
		if err == nil {
			t.Error("ParseRules() error = nil, want error")
		}
	})

	t.Run("Multiple rules", func(t *testing.T) {
		policy := &Policy{
			RulesJSON: `[{"nas_id":"AP-01","allowed_ssids":["WIFI-A"],"vlan_id":"100","session_timeout":3600},{"nas_id":"*","allowed_ssids":["*"]}]`,
		}
		err := policy.ParseRules()
		if err != nil {
			t.Errorf("ParseRules() error = %v, want nil", err)
		}
		if len(policy.Rules) != 2 {
			t.Errorf("Rules length = %d, want %d", len(policy.Rules), 2)
		}
	})
}

func TestPolicyEncodeRules(t *testing.T) {
	t.Run("Empty rules", func(t *testing.T) {
		policy := NewPolicy("imsi", "allow")
		err := policy.EncodeRules()
		if err != nil {
			t.Errorf("EncodeRules() error = %v, want nil", err)
		}
		if policy.RulesJSON != "[]" {
			t.Errorf("RulesJSON = %q, want %q", policy.RulesJSON, "[]")
		}
	})

	t.Run("With rules", func(t *testing.T) {
		policy := NewPolicy("imsi", "deny")
		policy.Rules = []PolicyRule{
			{NasID: "AP-01", AllowedSSIDs: []string{"GUEST-WIFI"}, VlanID: "200", SessionTimeout: 1800},
		}
		err := policy.EncodeRules()
		if err != nil {
			t.Errorf("EncodeRules() error = %v, want nil", err)
		}
		// JSON形式でエンコードされていることを確認
		if policy.RulesJSON == "" || policy.RulesJSON == "[]" {
			t.Errorf("RulesJSON should not be empty: %q", policy.RulesJSON)
		}
	})

	t.Run("Roundtrip", func(t *testing.T) {
		original := NewPolicy("imsi", "allow")
		original.Rules = []PolicyRule{
			{NasID: "AP-OFFICE-01", AllowedSSIDs: []string{"CORP-WIFI"}, VlanID: "100", SessionTimeout: 3600},
			{NasID: "*", AllowedSSIDs: []string{"*"}},
		}
		err := original.EncodeRules()
		if err != nil {
			t.Fatalf("EncodeRules() error = %v", err)
		}

		restored := &Policy{RulesJSON: original.RulesJSON}
		err = restored.ParseRules()
		if err != nil {
			t.Fatalf("ParseRules() error = %v", err)
		}

		if len(restored.Rules) != 2 {
			t.Errorf("Rules length = %d, want %d", len(restored.Rules), 2)
		}
		if restored.Rules[0].NasID != "AP-OFFICE-01" {
			t.Errorf("Rules[0].NasID = %q, want %q", restored.Rules[0].NasID, "AP-OFFICE-01")
		}
		if restored.Rules[1].NasID != "*" {
			t.Errorf("Rules[1].NasID = %q, want %q", restored.Rules[1].NasID, "*")
		}
	})
}

func TestPolicyIsAllowByDefault(t *testing.T) {
	tests := []struct {
		defaultAction string
		want          bool
	}{
		{"allow", true},
		{"deny", false},
		{"ALLOW", false}, // 大文字は一致しない
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.defaultAction, func(t *testing.T) {
			policy := NewPolicy("imsi", tt.defaultAction)
			if got := policy.IsAllowByDefault(); got != tt.want {
				t.Errorf("IsAllowByDefault() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPolicyRule(t *testing.T) {
	rule := PolicyRule{
		NasID:          "AP-OFFICE-01",
		AllowedSSIDs:   []string{"CORP-WIFI", "GUEST-WIFI"},
		VlanID:         "100",
		SessionTimeout: 3600,
	}

	if rule.NasID != "AP-OFFICE-01" {
		t.Errorf("NasID = %q, want %q", rule.NasID, "AP-OFFICE-01")
	}
	if len(rule.AllowedSSIDs) != 2 {
		t.Errorf("AllowedSSIDs length = %d, want %d", len(rule.AllowedSSIDs), 2)
	}
	if rule.VlanID != "100" {
		t.Errorf("VlanID = %q, want %q", rule.VlanID, "100")
	}
	if rule.SessionTimeout != 3600 {
		t.Errorf("SessionTimeout = %d, want %d", rule.SessionTimeout, 3600)
	}
}
