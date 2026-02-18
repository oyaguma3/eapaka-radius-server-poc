package milenage

import (
	"encoding/hex"
	"testing"
)

func TestSQNToBytes(t *testing.T) {
	tests := []struct {
		sqn  uint64
		want string
	}{
		{0, "000000000000"},
		{32, "000000000020"},
		{0xabcdef123456, "abcdef123456"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := SQNToBytes(tt.sqn)
			gotHex := hex.EncodeToString(got)
			if gotHex != tt.want {
				t.Errorf("SQNToBytes(%d) = %q, want %q", tt.sqn, gotHex, tt.want)
			}
		})
	}
}

func TestBytesToSQN(t *testing.T) {
	tests := []struct {
		hexStr string
		want   uint64
	}{
		{"000000000000", 0},
		{"000000000020", 32},
		{"abcdef123456", 0xabcdef123456},
	}

	for _, tt := range tests {
		t.Run(tt.hexStr, func(t *testing.T) {
			b, _ := hex.DecodeString(tt.hexStr)
			got := BytesToSQN(b)
			if got != tt.want {
				t.Errorf("BytesToSQN(%q) = %d, want %d", tt.hexStr, got, tt.want)
			}
		})
	}
}

func TestSQNRoundtrip(t *testing.T) {
	tests := []uint64{0, 1, 32, 255, 65535, 0xabcdef123456}

	for _, sqn := range tests {
		b := SQNToBytes(sqn)
		got := BytesToSQN(b)
		if got != sqn {
			t.Errorf("Roundtrip failed for %d: got %d", sqn, got)
		}
	}
}

func TestCalculatorGenerateVectorWithRAND(t *testing.T) {
	calc := NewCalculator()

	// 3GPP TS 35.208 テストベクター（Set 1）
	ki, _ := hex.DecodeString("465b5ce8b199b49faa5f0a2ee238a6bc")
	opc, _ := hex.DecodeString("cd63cb71954a9f4e48a5994e37a02baf")
	amf, _ := hex.DecodeString("8000")
	randVal, _ := hex.DecodeString("23553cbe9637a89d218ae64dae47bf35")
	sqn := uint64(0x000000000020)

	vector, err := calc.GenerateVectorWithRAND(ki, opc, amf, sqn, randVal)
	if err != nil {
		t.Fatalf("GenerateVectorWithRAND() error = %v", err)
	}

	// 基本的な長さチェック
	if len(vector.RAND) != 16 {
		t.Errorf("RAND length = %d, want 16", len(vector.RAND))
	}
	if len(vector.AUTN) != 16 {
		t.Errorf("AUTN length = %d, want 16", len(vector.AUTN))
	}
	if len(vector.XRES) != 8 {
		t.Errorf("XRES length = %d, want 8", len(vector.XRES))
	}
	if len(vector.CK) != 16 {
		t.Errorf("CK length = %d, want 16", len(vector.CK))
	}
	if len(vector.IK) != 16 {
		t.Errorf("IK length = %d, want 16", len(vector.IK))
	}
}

func TestCalculatorGenerateVector(t *testing.T) {
	calc := NewCalculator()

	ki, _ := hex.DecodeString("465b5ce8b199b49faa5f0a2ee238a6bc")
	opc, _ := hex.DecodeString("cd63cb71954a9f4e48a5994e37a02baf")
	amf, _ := hex.DecodeString("8000")
	sqn := uint64(0x000000000020)

	vector, err := calc.GenerateVector(ki, opc, amf, sqn)
	if err != nil {
		t.Fatalf("GenerateVector() error = %v", err)
	}

	// RANDがランダムに生成されていることを確認
	if len(vector.RAND) != 16 {
		t.Errorf("RAND length = %d, want 16", len(vector.RAND))
	}
}
