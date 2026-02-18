package eap

import (
	"testing"

	eapaka "github.com/oyaguma3/go-eapaka"
)

// buildTestChallengePacket はテスト用のEAP-AKA Challengeパケットを構築する
func buildTestChallengePacket(t *testing.T) []byte {
	t.Helper()
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeRequest,
		Identifier: 1,
		Type:       eapaka.TypeAKA,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRand{Rand: make([]byte, 16)},
			&eapaka.AtAutn{Autn: make([]byte, 16)},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}
	data, err := pkt.Marshal()
	if err != nil {
		t.Fatalf("テストパケット構築に失敗: %v", err)
	}
	return data
}

func TestParseEAPPacket_Valid(t *testing.T) {
	data := buildTestChallengePacket(t)

	pkt, err := ParseEAPPacket(data)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if pkt.Code != eapaka.CodeRequest {
		t.Errorf("Code: got %d, want %d", pkt.Code, eapaka.CodeRequest)
	}
	if pkt.Identifier != 1 {
		t.Errorf("Identifier: got %d, want 1", pkt.Identifier)
	}
	if pkt.Type != eapaka.TypeAKA {
		t.Errorf("Type: got %d, want %d", pkt.Type, eapaka.TypeAKA)
	}
	if pkt.Subtype != eapaka.SubtypeChallenge {
		t.Errorf("Subtype: got %d, want %d", pkt.Subtype, eapaka.SubtypeChallenge)
	}
}

func TestParseEAPPacket_InvalidData(t *testing.T) {
	// 不正なバイト列
	_, err := ParseEAPPacket([]byte{0xFF, 0xFF})
	if err == nil {
		t.Error("不正データでエラーが返るべき")
	}
}

func TestParseEAPPacket_EmptyData(t *testing.T) {
	_, err := ParseEAPPacket([]byte{})
	if err == nil {
		t.Error("空データでエラーが返るべき")
	}
}

func TestGetAttribute_Found(t *testing.T) {
	data := buildTestChallengePacket(t)
	pkt, err := ParseEAPPacket(data)
	if err != nil {
		t.Fatalf("パース失敗: %v", err)
	}

	rand, found := GetAttribute[*eapaka.AtRand](pkt)
	if !found {
		t.Fatal("AT_RANDが見つからない")
	}
	if len(rand.Rand) != 16 {
		t.Errorf("AT_RAND長: got %d, want 16", len(rand.Rand))
	}
}

func TestGetAttribute_NotFound(t *testing.T) {
	data := buildTestChallengePacket(t)
	pkt, err := ParseEAPPacket(data)
	if err != nil {
		t.Fatalf("パース失敗: %v", err)
	}

	// AT_RESは含まれていない
	_, found := GetAttribute[*eapaka.AtRes](pkt)
	if found {
		t.Error("AT_RESは含まれていないのにfound=true")
	}
}

func TestGetAttribute_EmptyAttributes(t *testing.T) {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeRequest,
		Identifier: 1,
		Type:       eapaka.TypeAKA,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: nil,
	}

	_, found := GetAttribute[*eapaka.AtRand](pkt)
	if found {
		t.Error("空属性リストでfound=true")
	}
}

func TestBuildEAPSuccess(t *testing.T) {
	data, err := BuildEAPSuccess(42)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(data) != 4 {
		t.Fatalf("長さ: got %d, want 4", len(data))
	}

	// 再パースで検証
	pkt, err := ParseEAPPacket(data)
	if err != nil {
		t.Fatalf("再パース失敗: %v", err)
	}
	if pkt.Code != eapaka.CodeSuccess {
		t.Errorf("Code: got %d, want %d", pkt.Code, eapaka.CodeSuccess)
	}
	if pkt.Identifier != 42 {
		t.Errorf("Identifier: got %d, want 42", pkt.Identifier)
	}
}

func TestBuildEAPFailure(t *testing.T) {
	data, err := BuildEAPFailure(99)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if len(data) != 4 {
		t.Fatalf("長さ: got %d, want 4", len(data))
	}

	// 再パースで検証
	pkt, err := ParseEAPPacket(data)
	if err != nil {
		t.Fatalf("再パース失敗: %v", err)
	}
	if pkt.Code != eapaka.CodeFailure {
		t.Errorf("Code: got %d, want %d", pkt.Code, eapaka.CodeFailure)
	}
	if pkt.Identifier != 99 {
		t.Errorf("Identifier: got %d, want 99", pkt.Identifier)
	}
}

func TestBuildAKAIdentityRequest_AKA(t *testing.T) {
	data, err := BuildAKAIdentityRequest(10, eapaka.TypeAKA)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	pkt, err := ParseEAPPacket(data)
	if err != nil {
		t.Fatalf("再パース失敗: %v", err)
	}
	if pkt.Code != eapaka.CodeRequest {
		t.Errorf("Code: got %d, want %d", pkt.Code, eapaka.CodeRequest)
	}
	if pkt.Identifier != 10 {
		t.Errorf("Identifier: got %d, want 10", pkt.Identifier)
	}
	if pkt.Type != eapaka.TypeAKA {
		t.Errorf("Type: got %d, want %d", pkt.Type, eapaka.TypeAKA)
	}
	if pkt.Subtype != eapaka.SubtypeIdentity {
		t.Errorf("Subtype: got %d, want %d", pkt.Subtype, eapaka.SubtypeIdentity)
	}

	// AT_PERMANENT_ID_REQの存在確認
	_, found := GetAttribute[*eapaka.AtPermanentIdReq](pkt)
	if !found {
		t.Error("AT_PERMANENT_ID_REQが見つからない")
	}
}

func TestGetEAPType_Valid(t *testing.T) {
	data := buildTestChallengePacket(t)

	got := GetEAPType(data)
	if got != eapaka.TypeAKA {
		t.Errorf("GetEAPType: got %d, want %d", got, eapaka.TypeAKA)
	}
}

func TestGetEAPType_ShortData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"空データ", []byte{}},
		{"1バイト", []byte{0x01}},
		{"4バイト", []byte{0x01, 0x02, 0x03, 0x04}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEAPType(tt.data)
			if got != 0 {
				t.Errorf("GetEAPType(%v): got %d, want 0", tt.data, got)
			}
		})
	}
}

func TestGetEAPIdentifier_Valid(t *testing.T) {
	data := buildTestChallengePacket(t)

	got := GetEAPIdentifier(data)
	if got != 1 {
		t.Errorf("GetEAPIdentifier: got %d, want 1", got)
	}
}

func TestGetEAPIdentifier_ShortData(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"空データ", []byte{}},
		{"1バイト", []byte{0x01}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEAPIdentifier(tt.data)
			if got != 0 {
				t.Errorf("GetEAPIdentifier(%v): got %d, want 0", tt.data, got)
			}
		})
	}
}

func TestBuildAKAIdentityRequest_AKAPrime(t *testing.T) {
	data, err := BuildAKAIdentityRequest(20, eapaka.TypeAKAPrime)
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}

	pkt, err := ParseEAPPacket(data)
	if err != nil {
		t.Fatalf("再パース失敗: %v", err)
	}
	if pkt.Code != eapaka.CodeRequest {
		t.Errorf("Code: got %d, want %d", pkt.Code, eapaka.CodeRequest)
	}
	if pkt.Identifier != 20 {
		t.Errorf("Identifier: got %d, want 20", pkt.Identifier)
	}
	if pkt.Type != eapaka.TypeAKAPrime {
		t.Errorf("Type: got %d, want %d", pkt.Type, eapaka.TypeAKAPrime)
	}
	if pkt.Subtype != eapaka.SubtypeIdentity {
		t.Errorf("Subtype: got %d, want %d", pkt.Subtype, eapaka.SubtypeIdentity)
	}

	_, found := GetAttribute[*eapaka.AtPermanentIdReq](pkt)
	if !found {
		t.Error("AT_PERMANENT_ID_REQが見つからない")
	}
}
