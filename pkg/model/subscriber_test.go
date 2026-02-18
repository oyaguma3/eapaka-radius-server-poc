package model

import "testing"

func TestNewSubscriber(t *testing.T) {
	sub := NewSubscriber(
		"440101234567890",
		"00112233445566778899aabbccddeeff",
		"aabbccddeeff00112233445566778899",
		"8000",
		"000000000001",
		"2024-01-01T00:00:00Z",
	)

	if sub.IMSI != "440101234567890" {
		t.Errorf("IMSI = %q, want %q", sub.IMSI, "440101234567890")
	}
	if sub.Ki != "00112233445566778899aabbccddeeff" {
		t.Errorf("Ki = %q, want %q", sub.Ki, "00112233445566778899aabbccddeeff")
	}
	if sub.OPc != "aabbccddeeff00112233445566778899" {
		t.Errorf("OPc = %q, want %q", sub.OPc, "aabbccddeeff00112233445566778899")
	}
	if sub.AMF != "8000" {
		t.Errorf("AMF = %q, want %q", sub.AMF, "8000")
	}
	if sub.SQN != "000000000001" {
		t.Errorf("SQN = %q, want %q", sub.SQN, "000000000001")
	}
	if sub.CreatedAt != "2024-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q, want %q", sub.CreatedAt, "2024-01-01T00:00:00Z")
	}
}

func TestSubscriberStruct(t *testing.T) {
	// 構造体を直接初期化
	sub := Subscriber{
		IMSI:      "440109876543210",
		Ki:        "ffeeddccbbaa99887766554433221100",
		OPc:       "00112233445566778899aabbccddeeff",
		AMF:       "0000",
		SQN:       "ffffffffffff",
		CreatedAt: "2024-12-31T23:59:59Z",
	}

	if sub.IMSI != "440109876543210" {
		t.Errorf("IMSI = %q, want %q", sub.IMSI, "440109876543210")
	}
}
