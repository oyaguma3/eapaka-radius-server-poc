package sqn

import "testing"

func TestManagerIncrement(t *testing.T) {
	m := NewManager()

	tests := []struct {
		name       string
		currentSQN uint64
		wantSQN    uint64
		wantErr    bool
	}{
		{"zero", 0, 32, false},
		{"normal", 32, 64, false},
		{"large", 1000000, 1000032, false},
		{"near max", MaxSQN - 32, MaxSQN, false},
		{"overflow", MaxSQN, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.Increment(tt.currentSQN)
			if (err != nil) != tt.wantErr {
				t.Errorf("Increment() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantSQN {
				t.Errorf("Increment() = %d, want %d", got, tt.wantSQN)
			}
		})
	}
}

func TestManagerGetSEQ(t *testing.T) {
	m := NewManager()

	tests := []struct {
		sqn     uint64
		wantSEQ uint64
	}{
		{0, 0},
		{32, 1},    // SEQ=1, IND=0
		{33, 1},    // SEQ=1, IND=1
		{64, 2},    // SEQ=2, IND=0
		{1000, 31}, // SEQ=31, IND=8
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := m.GetSEQ(tt.sqn)
			if got != tt.wantSEQ {
				t.Errorf("GetSEQ(%d) = %d, want %d", tt.sqn, got, tt.wantSEQ)
			}
		})
	}
}

func TestManagerGetIND(t *testing.T) {
	m := NewManager()

	tests := []struct {
		sqn     uint64
		wantIND uint8
	}{
		{0, 0},
		{1, 1},
		{31, 31},
		{32, 0},
		{33, 1},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := m.GetIND(tt.sqn)
			if got != tt.wantIND {
				t.Errorf("GetIND(%d) = %d, want %d", tt.sqn, got, tt.wantIND)
			}
		})
	}
}

func TestManagerFormatHex(t *testing.T) {
	m := NewManager()

	tests := []struct {
		sqn  uint64
		want string
	}{
		{0, "000000000000"},
		{32, "000000000020"},
		{255, "0000000000ff"},
		{0xabcdef123456, "abcdef123456"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := m.FormatHex(tt.sqn)
			if got != tt.want {
				t.Errorf("FormatHex(%d) = %q, want %q", tt.sqn, got, tt.want)
			}
		})
	}
}

func TestManagerParseHex(t *testing.T) {
	m := NewManager()

	tests := []struct {
		s       string
		want    uint64
		wantErr bool
	}{
		{"000000000000", 0, false},
		{"000000000020", 32, false},
		{"abcdef123456", 0xabcdef123456, false},
		{"ABCDEF123456", 0xabcdef123456, false},
		{"00000000000", 0, true},   // 11桁
		{"0000000000000", 0, true}, // 13桁
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			got, err := m.ParseHex(tt.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHex() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseHex(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}
