package testmode

import "testing"

func TestIsTestIMSI(t *testing.T) {
	p := NewTestVectorProvider("00101")

	tests := []struct {
		imsi string
		want bool
	}{
		{"001010000000001", true},
		{"001019999999999", true},
		{"440101234567890", false},
		{"00101", true},
		{"0010", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.imsi, func(t *testing.T) {
			got := p.IsTestIMSI(tt.imsi)
			if got != tt.want {
				t.Errorf("IsTestIMSI(%q) = %v, want %v", tt.imsi, got, tt.want)
			}
		})
	}
}

func TestGetTestVector(t *testing.T) {
	p := NewTestVectorProvider("00101")

	t.Run("valid test IMSI", func(t *testing.T) {
		vector, err := p.GetTestVector("001010000000001")
		if err != nil {
			t.Fatalf("GetTestVector() error = %v", err)
		}

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
	})

	t.Run("non-test IMSI", func(t *testing.T) {
		_, err := p.GetTestVector("440101234567890")
		if err == nil {
			t.Error("GetTestVector() expected error for non-test IMSI")
		}
	})
}

func TestGetTestCryptoParams(t *testing.T) {
	p := NewTestVectorProvider("00101")

	ki, opc, amf := p.GetTestCryptoParams()

	// サイズ検証
	if len(ki) != 16 {
		t.Errorf("Ki length = %d, want 16", len(ki))
	}
	if len(opc) != 16 {
		t.Errorf("OPc length = %d, want 16", len(opc))
	}
	if len(amf) != 2 {
		t.Errorf("AMF length = %d, want 2", len(amf))
	}

	// 防御的コピーの検証: 返却値を変更しても次回呼び出しに影響しない
	ki[0] = 0xFF
	opc[0] = 0xFF
	amf[0] = 0xFF

	ki2, opc2, amf2 := p.GetTestCryptoParams()

	if ki2[0] == 0xFF {
		t.Error("Ki is not a defensive copy")
	}
	if opc2[0] == 0xFF {
		t.Error("OPc is not a defensive copy")
	}
	if amf2[0] == 0xFF {
		t.Error("AMF is not a defensive copy")
	}
}

func TestGetDefaultSQN(t *testing.T) {
	p := NewTestVectorProvider("00101")

	sqn := p.GetDefaultSQN()

	expectedSQN := uint64(0xff9bb4d0b607)
	if sqn != expectedSQN {
		t.Errorf("GetDefaultSQN() = 0x%x, want 0x%x", sqn, expectedSQN)
	}
}
