package logging

import "testing"

func TestMaskIMSI_Normal(t *testing.T) {
	// 15桁IMSI: 先頭6 + 8個の'*' + 末尾1
	result := MaskIMSI("001010123456789", true)
	expected := "001010********9"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestMaskIMSI_Short(t *testing.T) {
	// 7文字以下はマスキングしない
	result := MaskIMSI("1234567", true)
	if result != "1234567" {
		t.Errorf("got %q, want %q", result, "1234567")
	}
}

func TestMaskIMSI_Disabled(t *testing.T) {
	// 無効化時はそのまま返す
	result := MaskIMSI("001010123456789", false)
	if result != "001010123456789" {
		t.Errorf("got %q, want %q", result, "001010123456789")
	}
}

func TestMaskIMSI_Empty(t *testing.T) {
	result := MaskIMSI("", true)
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestMaskIMSI_EightChars(t *testing.T) {
	// 8文字: 先頭6 + '*' + 末尾1
	result := MaskIMSI("12345678", true)
	expected := "123456*8"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}
