package milenage

import "testing"

func TestHexDecode(t *testing.T) {
	tests := []struct {
		input   string
		wantLen int
		wantErr bool
	}{
		{"00112233", 4, false},
		{"abcdef", 3, false},
		{"ABCDEF", 3, false},
		{"invalid", 0, true},
		{"", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := HexDecode(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("HexDecode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(got) != tt.wantLen {
				t.Errorf("HexDecode() len = %d, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestHexEncode(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte{0x00, 0x11, 0x22, 0x33}, "00112233"},
		{[]byte{0xab, 0xcd, 0xef}, "abcdef"},
		{[]byte{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := HexEncode(tt.input)
			if got != tt.want {
				t.Errorf("HexEncode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVectorToResponse(t *testing.T) {
	vector := &Vector{
		RAND: []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 0x10},
		AUTN: []byte{0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20},
		XRES: []byte{0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28},
		CK:   []byte{0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f, 0x40},
		IK:   []byte{0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f, 0x50},
	}

	resp := VectorToResponse(vector)

	if resp.RAND != "0102030405060708090a0b0c0d0e0f10" {
		t.Errorf("RAND = %q, unexpected", resp.RAND)
	}
	if len(resp.AUTN) != 32 {
		t.Errorf("AUTN length = %d, want 32", len(resp.AUTN))
	}
	if len(resp.XRES) != 16 {
		t.Errorf("XRES length = %d, want 16", len(resp.XRES))
	}
	if len(resp.CK) != 32 {
		t.Errorf("CK length = %d, want 32", len(resp.CK))
	}
	if len(resp.IK) != 32 {
		t.Errorf("IK length = %d, want 32", len(resp.IK))
	}
}
