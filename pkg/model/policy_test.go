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
			RulesJSON: `[{"ssid":"test-wifi","action":"allow","time_min":"09:00","time_max":"18:00"}]`,
		}
		err := policy.ParseRules()
		if err != nil {
			t.Errorf("ParseRules() error = %v, want nil", err)
		}
		if len(policy.Rules) != 1 {
			t.Fatalf("Rules length = %d, want %d", len(policy.Rules), 1)
		}
		rule := policy.Rules[0]
		if rule.SSID != "test-wifi" {
			t.Errorf("SSID = %q, want %q", rule.SSID, "test-wifi")
		}
		if rule.Action != "allow" {
			t.Errorf("Action = %q, want %q", rule.Action, "allow")
		}
		if rule.TimeMin != "09:00" {
			t.Errorf("TimeMin = %q, want %q", rule.TimeMin, "09:00")
		}
		if rule.TimeMax != "18:00" {
			t.Errorf("TimeMax = %q, want %q", rule.TimeMax, "18:00")
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
			RulesJSON: `[{"ssid":"wifi-a","action":"allow","time_min":"","time_max":""},{"ssid":"wifi-b","action":"deny","time_min":"00:00","time_max":"06:00"}]`,
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
			{SSID: "guest-wifi", Action: "allow", TimeMin: "", TimeMax: ""},
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
			{SSID: "corp-wifi", Action: "allow", TimeMin: "08:00", TimeMax: "20:00"},
			{SSID: "*", Action: "deny", TimeMin: "", TimeMax: ""},
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
		if restored.Rules[0].SSID != "corp-wifi" {
			t.Errorf("Rules[0].SSID = %q, want %q", restored.Rules[0].SSID, "corp-wifi")
		}
		if restored.Rules[1].SSID != "*" {
			t.Errorf("Rules[1].SSID = %q, want %q", restored.Rules[1].SSID, "*")
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
		SSID:    "test-ssid",
		Action:  "allow",
		TimeMin: "09:00",
		TimeMax: "17:00",
	}

	if rule.SSID != "test-ssid" {
		t.Errorf("SSID = %q, want %q", rule.SSID, "test-ssid")
	}
	if rule.Action != "allow" {
		t.Errorf("Action = %q, want %q", rule.Action, "allow")
	}
	if rule.TimeMin != "09:00" {
		t.Errorf("TimeMin = %q, want %q", rule.TimeMin, "09:00")
	}
	if rule.TimeMax != "17:00" {
		t.Errorf("TimeMax = %q, want %q", rule.TimeMax, "17:00")
	}
}
