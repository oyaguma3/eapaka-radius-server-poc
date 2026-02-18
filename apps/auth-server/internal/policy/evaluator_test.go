package policy

import "testing"

func TestEvaluateMatchFirstRule(t *testing.T) {
	p := &Policy{
		Rules: []PolicyRule{
			{NasID: "nas-01", AllowedSSIDs: []string{"SSID-A"}},
			{NasID: "nas-02", AllowedSSIDs: []string{"SSID-B"}},
		},
		Default: "deny",
	}
	e := NewEvaluator()
	result := e.Evaluate(p, "nas-01", "SSID-A")
	if !result.Allowed {
		t.Fatal("expected Allowed=true")
	}
	if result.MatchedRule == nil {
		t.Fatal("expected MatchedRule to be non-nil")
	}
	if result.MatchedRule.NasID != "nas-01" {
		t.Errorf("MatchedRule.NasID = %q, want %q", result.MatchedRule.NasID, "nas-01")
	}
}

func TestEvaluateMatchSecondRule(t *testing.T) {
	p := &Policy{
		Rules: []PolicyRule{
			{NasID: "nas-01", AllowedSSIDs: []string{"SSID-A"}},
			{NasID: "nas-02", AllowedSSIDs: []string{"SSID-B"}, VlanID: "200"},
		},
		Default: "deny",
	}
	e := NewEvaluator()
	result := e.Evaluate(p, "nas-02", "SSID-B")
	if !result.Allowed {
		t.Fatal("expected Allowed=true")
	}
	if result.MatchedRule == nil {
		t.Fatal("expected MatchedRule to be non-nil")
	}
	if result.MatchedRule.VlanID != "200" {
		t.Errorf("MatchedRule.VlanID = %q, want %q", result.MatchedRule.VlanID, "200")
	}
}

func TestEvaluateNoMatchDefaultAllow(t *testing.T) {
	p := &Policy{
		Rules: []PolicyRule{
			{NasID: "nas-01", AllowedSSIDs: []string{"SSID-A"}},
		},
		Default: "allow",
	}
	e := NewEvaluator()
	result := e.Evaluate(p, "nas-99", "SSID-X")
	if !result.Allowed {
		t.Fatal("expected Allowed=true with default=allow")
	}
	if result.MatchedRule != nil {
		t.Error("expected MatchedRule to be nil for default match")
	}
}

func TestEvaluateNoMatchDefaultDeny(t *testing.T) {
	p := &Policy{
		Rules: []PolicyRule{
			{NasID: "nas-01", AllowedSSIDs: []string{"SSID-A"}},
		},
		Default: "deny",
	}
	e := NewEvaluator()
	result := e.Evaluate(p, "nas-99", "SSID-X")
	if result.Allowed {
		t.Fatal("expected Allowed=false with default=deny")
	}
	if result.DenyReason == "" {
		t.Error("expected DenyReason to be set")
	}
}

func TestEvaluateWildcardSSID(t *testing.T) {
	p := &Policy{
		Rules: []PolicyRule{
			{NasID: "nas-01", AllowedSSIDs: []string{"*"}},
		},
		Default: "deny",
	}
	e := NewEvaluator()
	result := e.Evaluate(p, "nas-01", "ANY-SSID")
	if !result.Allowed {
		t.Fatal("expected Allowed=true with wildcard SSID")
	}
	if result.MatchedRule == nil {
		t.Fatal("expected MatchedRule to be non-nil")
	}
}

func TestEvaluateSSIDCaseInsensitive(t *testing.T) {
	p := &Policy{
		Rules: []PolicyRule{
			{NasID: "nas-01", AllowedSSIDs: []string{"My-SSID"}},
		},
		Default: "deny",
	}
	e := NewEvaluator()
	result := e.Evaluate(p, "nas-01", "my-ssid")
	if !result.Allowed {
		t.Fatal("expected Allowed=true (SSID comparison should be case-insensitive)")
	}
}

func TestEvaluateNasIDCaseSensitive(t *testing.T) {
	p := &Policy{
		Rules: []PolicyRule{
			{NasID: "NAS-01", AllowedSSIDs: []string{"SSID-A"}},
		},
		Default: "deny",
	}
	e := NewEvaluator()
	// 小文字のnas-01は大文字のNAS-01と一致しない
	result := e.Evaluate(p, "nas-01", "SSID-A")
	if result.Allowed {
		t.Fatal("expected Allowed=false (NAS-ID comparison should be case-sensitive)")
	}
}

func TestEvaluateEmptyRules(t *testing.T) {
	p := &Policy{
		Rules:   []PolicyRule{},
		Default: "deny",
	}
	e := NewEvaluator()
	result := e.Evaluate(p, "nas-01", "SSID-A")
	if result.Allowed {
		t.Fatal("expected Allowed=false with empty rules and default=deny")
	}

	// default=allowの場合
	p.Default = "allow"
	result = e.Evaluate(p, "nas-01", "SSID-A")
	if !result.Allowed {
		t.Fatal("expected Allowed=true with empty rules and default=allow")
	}
}

func TestExtractSSID(t *testing.T) {
	ssid := ExtractSSID("AA-BB-CC-DD-EE-FF:MyNetwork")
	if ssid != "MyNetwork" {
		t.Errorf("ExtractSSID = %q, want %q", ssid, "MyNetwork")
	}
}

func TestExtractSSIDNoColon(t *testing.T) {
	ssid := ExtractSSID("MyNetwork")
	if ssid != "MyNetwork" {
		t.Errorf("ExtractSSID = %q, want %q", ssid, "MyNetwork")
	}
}

func TestExtractSSIDEmpty(t *testing.T) {
	ssid := ExtractSSID("")
	if ssid != "" {
		t.Errorf("ExtractSSID = %q, want empty string", ssid)
	}
}
