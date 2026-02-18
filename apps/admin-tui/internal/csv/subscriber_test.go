package csv

import (
	"bytes"
	"strings"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestParseSubscriberCSV_ValidData(t *testing.T) {
	csvData := `imsi,ki,opc,amf,sqn
440101234567890,465b5ce8b199b49faa5f0a2ee238a6bc,cd63cb71954a9f4e48a5994e37a02baf,8000,000000000020
440109876543210,0123456789abcdef0123456789abcdef,fedcba9876543210fedcba9876543210,8000,000000000001`

	subscribers, errs := ParseSubscriberCSV(strings.NewReader(csvData))

	if len(errs) > 0 {
		t.Errorf("ParseSubscriberCSV() errors = %v, want no errors", errs)
	}

	if len(subscribers) != 2 {
		t.Fatalf("ParseSubscriberCSV() got %d subscribers, want 2", len(subscribers))
	}

	// 1件目の検証
	if subscribers[0].IMSI != "440101234567890" {
		t.Errorf("subscribers[0].IMSI = %q, want %q", subscribers[0].IMSI, "440101234567890")
	}
	// Ki, OPcは正規化により大文字に変換される
	if subscribers[0].Ki != "465B5CE8B199B49FAA5F0A2EE238A6BC" {
		t.Errorf("subscribers[0].Ki = %q, want %q", subscribers[0].Ki, "465B5CE8B199B49FAA5F0A2EE238A6BC")
	}
}

func TestParseSubscriberCSV_InvalidHeader(t *testing.T) {
	tests := []struct {
		name    string
		csvData string
	}{
		{
			name:    "Wrong header order",
			csvData: "ki,imsi,opc,amf,sqn\n",
		},
		{
			name:    "Missing columns",
			csvData: "imsi,ki\n",
		},
		{
			name:    "Empty file",
			csvData: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := ParseSubscriberCSV(strings.NewReader(tt.csvData))
			if len(errs) == 0 {
				t.Error("ParseSubscriberCSV() expected error, got none")
			}
		})
	}
}

func TestParseSubscriberCSV_InvalidData(t *testing.T) {
	tests := []struct {
		name    string
		csvData string
	}{
		{
			name: "Invalid IMSI (too short)",
			csvData: `imsi,ki,opc,amf,sqn
12345,465b5ce8b199b49faa5f0a2ee238a6bc,cd63cb71954a9f4e48a5994e37a02baf,8000,000000000020`,
		},
		{
			name: "Invalid Ki (wrong length)",
			csvData: `imsi,ki,opc,amf,sqn
440101234567890,abcdef,cd63cb71954a9f4e48a5994e37a02baf,8000,000000000020`,
		},
		{
			name: "Missing columns in data row",
			csvData: `imsi,ki,opc,amf,sqn
440101234567890,465b5ce8b199b49faa5f0a2ee238a6bc`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, errs := ParseSubscriberCSV(strings.NewReader(tt.csvData))
			if len(errs) == 0 {
				t.Error("ParseSubscriberCSV() expected error, got none")
			}
		})
	}
}

func TestParseSubscriberCSV_PartialErrors(t *testing.T) {
	// 1行目は正常、2行目はエラー
	csvData := `imsi,ki,opc,amf,sqn
440101234567890,465b5ce8b199b49faa5f0a2ee238a6bc,cd63cb71954a9f4e48a5994e37a02baf,8000,000000000020
invalid,invalid,invalid,invalid,invalid`

	subscribers, errs := ParseSubscriberCSV(strings.NewReader(csvData))

	if len(errs) == 0 {
		t.Error("Expected errors for invalid row")
	}
	if len(subscribers) != 1 {
		t.Errorf("Expected 1 valid subscriber, got %d", len(subscribers))
	}
}

func TestWriteSubscriberCSV(t *testing.T) {
	subscribers := []*model.Subscriber{
		{
			IMSI: "440101234567890",
			Ki:   "465b5ce8b199b49faa5f0a2ee238a6bc",
			OPc:  "cd63cb71954a9f4e48a5994e37a02baf",
			AMF:  "8000",
			SQN:  "000000000020",
		},
		{
			IMSI: "440109876543210",
			Ki:   "0123456789abcdef0123456789abcdef",
			OPc:  "fedcba9876543210fedcba9876543210",
			AMF:  "8000",
			SQN:  "000000000001",
		},
	}

	var buf bytes.Buffer
	err := WriteSubscriberCSV(&buf, subscribers)
	if err != nil {
		t.Fatalf("WriteSubscriberCSV() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 3 { // ヘッダー + 2データ行
		t.Errorf("Expected 3 lines, got %d", len(lines))
	}

	// ヘッダー検証
	expectedHeader := "imsi,ki,opc,amf,sqn"
	if lines[0] != expectedHeader {
		t.Errorf("Header = %q, want %q", lines[0], expectedHeader)
	}

	// 1件目のデータ検証
	if !strings.Contains(lines[1], "440101234567890") {
		t.Errorf("First data row should contain IMSI: %s", lines[1])
	}
}

func TestWriteSubscriberCSV_EmptyList(t *testing.T) {
	var buf bytes.Buffer
	err := WriteSubscriberCSV(&buf, []*model.Subscriber{})
	if err != nil {
		t.Fatalf("WriteSubscriberCSV() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) != 1 { // ヘッダーのみ
		t.Errorf("Expected 1 line (header only), got %d", len(lines))
	}
}

func TestSubscriberCSV_Roundtrip(t *testing.T) {
	// 正規化後の大文字でテストデータを作成
	original := []*model.Subscriber{
		{
			IMSI: "440101234567890",
			Ki:   "465B5CE8B199B49FAA5F0A2EE238A6BC",
			OPc:  "CD63CB71954A9F4E48A5994E37A02BAF",
			AMF:  "8000",
			SQN:  "000000000020",
		},
	}

	// Write
	var buf bytes.Buffer
	if err := WriteSubscriberCSV(&buf, original); err != nil {
		t.Fatalf("WriteSubscriberCSV() error = %v", err)
	}

	// Read back
	parsed, errs := ParseSubscriberCSV(strings.NewReader(buf.String()))
	if len(errs) > 0 {
		t.Fatalf("ParseSubscriberCSV() errors = %v", errs)
	}

	if len(parsed) != len(original) {
		t.Fatalf("Roundtrip: got %d subscribers, want %d", len(parsed), len(original))
	}

	if parsed[0].IMSI != original[0].IMSI {
		t.Errorf("Roundtrip IMSI = %q, want %q", parsed[0].IMSI, original[0].IMSI)
	}
	if parsed[0].Ki != original[0].Ki {
		t.Errorf("Roundtrip Ki = %q, want %q", parsed[0].Ki, original[0].Ki)
	}
}
