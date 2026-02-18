package csv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestParseClientCSV_ValidData(t *testing.T) {
	csvData := `ip,secret,name,vendor
192.168.1.1,secret123,AP-001,Cisco
10.0.0.1,secret456,AP-002,`

	clients, errs := ParseClientCSV(strings.NewReader(csvData))

	if len(errs) > 0 {
		t.Errorf("ParseClientCSV() errors = %v, want no errors", errs)
	}

	if len(clients) != 2 {
		t.Fatalf("ParseClientCSV() got %d clients, want 2", len(clients))
	}

	// 1件目の検証
	if clients[0].IP != "192.168.1.1" {
		t.Errorf("clients[0].IP = %q, want %q", clients[0].IP, "192.168.1.1")
	}
	if clients[0].Secret != "secret123" {
		t.Errorf("clients[0].Secret = %q, want %q", clients[0].Secret, "secret123")
	}
	if clients[0].Name != "AP-001" {
		t.Errorf("clients[0].Name = %q, want %q", clients[0].Name, "AP-001")
	}
	if clients[0].Vendor != "Cisco" {
		t.Errorf("clients[0].Vendor = %q, want %q", clients[0].Vendor, "Cisco")
	}

	// 2件目（vendorが空）の検証
	if clients[1].Vendor != "" {
		t.Errorf("clients[1].Vendor = %q, want empty string", clients[1].Vendor)
	}
}

func TestParseClientCSV_MinimalColumns(t *testing.T) {
	// vendorカラムなし（最小構成）
	csvData := `ip,secret,name
192.168.1.1,secret123,AP-001`

	clients, errs := ParseClientCSV(strings.NewReader(csvData))

	if len(errs) > 0 {
		t.Errorf("ParseClientCSV() errors = %v, want no errors", errs)
	}

	if len(clients) != 1 {
		t.Fatalf("ParseClientCSV() got %d clients, want 1", len(clients))
	}
}

func TestParseClientCSV_InvalidHeader(t *testing.T) {
	tests := []struct {
		name    string
		csvData string
	}{
		{
			name:    "Wrong header order",
			csvData: "secret,ip,name\n",
		},
		{
			name:    "Missing required columns",
			csvData: "ip,secret\n",
		},
		{
			name:    "Empty file",
			csvData: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := ParseClientCSV(strings.NewReader(tt.csvData))
			if len(errs) == 0 {
				t.Error("ParseClientCSV() expected error, got none")
			}
		})
	}
}

func TestParseClientCSV_InvalidData(t *testing.T) {
	tests := []struct {
		name    string
		csvData string
	}{
		{
			name: "Invalid IP format",
			csvData: `ip,secret,name
not-an-ip,secret123,AP-001`,
		},
		{
			name: "Empty secret",
			csvData: `ip,secret,name
192.168.1.1,,AP-001`,
		},
		{
			name: "Missing columns in data row",
			csvData: `ip,secret,name
192.168.1.1,secret123`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := ParseClientCSV(strings.NewReader(tt.csvData))
			if len(errs) == 0 {
				t.Error("ParseClientCSV() expected error, got none")
			}
		})
	}
}

func TestParseClientCSV_PartialErrors(t *testing.T) {
	// 1行目は正常、2行目はエラー
	csvData := `ip,secret,name,vendor
192.168.1.1,secret123,AP-001,Cisco
invalid-ip,secret456,AP-002,`

	clients, errs := ParseClientCSV(strings.NewReader(csvData))

	if len(errs) == 0 {
		t.Error("Expected errors for invalid row")
	}
	if len(clients) != 1 {
		t.Errorf("Expected 1 valid client, got %d", len(clients))
	}
}

func TestWriteClientCSV(t *testing.T) {
	clients := []*model.RadiusClient{
		{
			IP:     "192.168.1.1",
			Secret: "secret123",
			Name:   "AP-001",
			Vendor: "Cisco",
		},
		{
			IP:     "10.0.0.1",
			Secret: "secret456",
			Name:   "AP-002",
			Vendor: "",
		},
	}

	var buf bytes.Buffer
	err := WriteClientCSV(&buf, clients)
	if err != nil {
		t.Fatalf("WriteClientCSV() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 { // ヘッダー + 2データ行
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// ヘッダー検証
	expectedHeader := "ip,secret,name,vendor"
	if lines[0] != expectedHeader {
		t.Errorf("Header = %q, want %q", lines[0], expectedHeader)
	}

	// 1件目のデータ検証
	if !strings.Contains(lines[1], "192.168.1.1") {
		t.Errorf("First data row should contain IP: %s", lines[1])
	}
}

func TestWriteClientCSV_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	err := WriteClientCSV(&buf, []*model.RadiusClient{})
	if err != nil {
		t.Fatalf("WriteClientCSV() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 1 { // ヘッダーのみ
		t.Errorf("Expected 1 line (header only), got %d", len(lines))
	}
}

func TestClientCSV_Roundtrip(t *testing.T) {
	original := []*model.RadiusClient{
		{
			IP:     "192.168.1.1",
			Secret: "secret123",
			Name:   "AP-001",
			Vendor: "Cisco",
		},
	}

	// Write
	var buf bytes.Buffer
	if err := WriteClientCSV(&buf, original); err != nil {
		t.Fatalf("WriteClientCSV() error = %v", err)
	}

	// Read back
	parsed, errs := ParseClientCSV(strings.NewReader(buf.String()))
	if len(errs) > 0 {
		t.Fatalf("ParseClientCSV() errors = %v", errs)
	}

	if len(parsed) != len(original) {
		t.Fatalf("Roundtrip: got %d clients, want %d", len(parsed), len(original))
	}

	if parsed[0].IP != original[0].IP {
		t.Errorf("Roundtrip IP = %q, want %q", parsed[0].IP, original[0].IP)
	}
	if parsed[0].Secret != original[0].Secret {
		t.Errorf("Roundtrip Secret = %q, want %q", parsed[0].Secret, original[0].Secret)
	}
}
