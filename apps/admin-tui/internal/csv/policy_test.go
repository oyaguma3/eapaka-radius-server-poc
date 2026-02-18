package csv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/model"
)

func TestParsePolicyCSV_ValidData(t *testing.T) {
	csvData := `imsi,default,rules_json
440101234567890,allow,[]
440109876543210,deny,"[{""nas_id"":""*"",""allowed_ssids"":[""SSID1"",""SSID2""],""vlan_id"":""100""}]"`

	policies, errs := ParsePolicyCSV(strings.NewReader(csvData))

	if len(errs) > 0 {
		t.Errorf("ParsePolicyCSV() errors = %v, want no errors", errs)
	}

	if len(policies) != 2 {
		t.Fatalf("ParsePolicyCSV() got %d policies, want 2", len(policies))
	}

	// 1件目の検証
	if policies[0].IMSI != "440101234567890" {
		t.Errorf("policies[0].IMSI = %q, want %q", policies[0].IMSI, "440101234567890")
	}
	if policies[0].Default != "allow" {
		t.Errorf("policies[0].Default = %q, want %q", policies[0].Default, "allow")
	}

	// 2件目の検証（ルールあり）
	if policies[1].IMSI != "440109876543210" {
		t.Errorf("policies[1].IMSI = %q, want %q", policies[1].IMSI, "440109876543210")
	}
	if len(policies[1].Rules) != 1 {
		t.Errorf("policies[1].Rules length = %d, want 1", len(policies[1].Rules))
	}
}

func TestParsePolicyCSV_EmptyRules(t *testing.T) {
	csvData := `imsi,default,rules_json
440101234567890,allow,
440109876543210,deny,[]`

	policies, errs := ParsePolicyCSV(strings.NewReader(csvData))

	if len(errs) > 0 {
		t.Errorf("ParsePolicyCSV() errors = %v, want no errors", errs)
	}

	if len(policies) != 2 {
		t.Fatalf("ParsePolicyCSV() got %d policies, want 2", len(policies))
	}

	// 空のルール
	if len(policies[0].Rules) != 0 {
		t.Errorf("policies[0].Rules should be empty, got %d", len(policies[0].Rules))
	}
	if len(policies[1].Rules) != 0 {
		t.Errorf("policies[1].Rules should be empty, got %d", len(policies[1].Rules))
	}
}

func TestParsePolicyCSV_InvalidHeader(t *testing.T) {
	tests := []struct {
		name    string
		csvData string
	}{
		{
			name:    "Wrong header order",
			csvData: "default,imsi,rules_json\n",
		},
		{
			name:    "Missing columns",
			csvData: "imsi,default\n",
		},
		{
			name:    "Empty file",
			csvData: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := ParsePolicyCSV(strings.NewReader(tt.csvData))
			if len(errs) == 0 {
				t.Error("ParsePolicyCSV() expected error, got none")
			}
		})
	}
}

func TestParsePolicyCSV_InvalidData(t *testing.T) {
	tests := []struct {
		name    string
		csvData string
	}{
		{
			name: "Invalid IMSI",
			csvData: `imsi,default,rules_json
12345,allow,[]`,
		},
		{
			name: "Invalid default action",
			csvData: `imsi,default,rules_json
440101234567890,invalid,[]`,
		},
		{
			name: "Invalid JSON in rules",
			csvData: `imsi,default,rules_json
440101234567890,allow,{invalid json}`,
		},
		{
			name: "Missing columns in data row",
			csvData: `imsi,default,rules_json
440101234567890,allow`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := ParsePolicyCSV(strings.NewReader(tt.csvData))
			if len(errs) == 0 {
				t.Error("ParsePolicyCSV() expected error, got none")
			}
		})
	}
}

func TestParsePolicyCSV_PartialErrors(t *testing.T) {
	// 1行目は正常、2行目はエラー
	csvData := `imsi,default,rules_json
440101234567890,allow,[]
invalid,invalid,invalid`

	policies, errs := ParsePolicyCSV(strings.NewReader(csvData))

	if len(errs) == 0 {
		t.Error("Expected errors for invalid row")
	}
	if len(policies) != 1 {
		t.Errorf("Expected 1 valid policy, got %d", len(policies))
	}
}

func TestWritePolicyCSV(t *testing.T) {
	policies := []*model.Policy{
		{
			IMSI:      "440101234567890",
			Default:   "allow",
			RulesJSON: "[]",
			Rules:     []model.PolicyRule{},
		},
		{
			IMSI:    "440109876543210",
			Default: "deny",
			Rules: []model.PolicyRule{
				{
					NasID:        "*",
					AllowedSSIDs: []string{"SSID1", "SSID2"},
					VlanID:       "100",
				},
			},
		},
	}

	var buf bytes.Buffer
	err := WritePolicyCSV(&buf, policies)
	if err != nil {
		t.Fatalf("WritePolicyCSV() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 { // ヘッダー + 2データ行
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// ヘッダー検証
	expectedHeader := "imsi,default,rules_json"
	if lines[0] != expectedHeader {
		t.Errorf("Header = %q, want %q", lines[0], expectedHeader)
	}

	// 1件目のデータ検証
	if !strings.Contains(lines[1], "440101234567890") {
		t.Errorf("First data row should contain IMSI: %s", lines[1])
	}
}

func TestWritePolicyCSV_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	err := WritePolicyCSV(&buf, []*model.Policy{})
	if err != nil {
		t.Fatalf("WritePolicyCSV() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 1 { // ヘッダーのみ
		t.Errorf("Expected 1 line (header only), got %d", len(lines))
	}
}

func TestPolicyCSV_Roundtrip(t *testing.T) {
	original := []*model.Policy{
		{
			IMSI:      "440101234567890",
			Default:   "allow",
			RulesJSON: "[]",
			Rules:     []model.PolicyRule{},
		},
	}

	// Write
	var buf bytes.Buffer
	if err := WritePolicyCSV(&buf, original); err != nil {
		t.Fatalf("WritePolicyCSV() error = %v", err)
	}

	// Read back
	parsed, errs := ParsePolicyCSV(strings.NewReader(buf.String()))
	if len(errs) > 0 {
		t.Fatalf("ParsePolicyCSV() errors = %v", errs)
	}

	if len(parsed) != len(original) {
		t.Fatalf("Roundtrip: got %d policies, want %d", len(parsed), len(original))
	}

	if parsed[0].IMSI != original[0].IMSI {
		t.Errorf("Roundtrip IMSI = %q, want %q", parsed[0].IMSI, original[0].IMSI)
	}
	if parsed[0].Default != original[0].Default {
		t.Errorf("Roundtrip Default = %q, want %q", parsed[0].Default, original[0].Default)
	}
}

func TestPolicyCSV_RoundtripWithRules(t *testing.T) {
	original := []*model.Policy{
		{
			IMSI:    "440101234567890",
			Default: "deny",
			Rules: []model.PolicyRule{
				{
					NasID:          "nas-001",
					AllowedSSIDs:   []string{"SSID1"},
					VlanID:         "100",
					SessionTimeout: 3600,
				},
			},
		},
	}

	// Write
	var buf bytes.Buffer
	if err := WritePolicyCSV(&buf, original); err != nil {
		t.Fatalf("WritePolicyCSV() error = %v", err)
	}

	// Read back
	parsed, errs := ParsePolicyCSV(strings.NewReader(buf.String()))
	if len(errs) > 0 {
		t.Fatalf("ParsePolicyCSV() errors = %v", errs)
	}

	if len(parsed) != 1 {
		t.Fatalf("Expected 1 policy, got %d", len(parsed))
	}

	if len(parsed[0].Rules) != 1 {
		t.Fatalf("Expected 1 rule, got %d", len(parsed[0].Rules))
	}

	if parsed[0].Rules[0].NasID != "nas-001" {
		t.Errorf("Rule NasID = %q, want %q", parsed[0].Rules[0].NasID, "nas-001")
	}
	if parsed[0].Rules[0].VlanID != "100" {
		t.Errorf("Rule VlanID = %q, want %q", parsed[0].Rules[0].VlanID, "100")
	}
}
