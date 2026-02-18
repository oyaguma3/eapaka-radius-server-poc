package sqn

import "testing"

func TestValidatorValidateResyncSQN(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		sqnMS   uint64
		sqnHE   uint64
		wantErr bool
	}{
		{"valid small diff", 100, 50, false},
		{"valid delta boundary", Delta + 100, 100, false},
		{"sqnMS equal", 100, 100, true},
		{"sqnMS less", 50, 100, true},
		{"delta exceeded", Delta + 101, 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateResyncSQN(tt.sqnMS, tt.sqnHE)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateResyncSQN() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatorComputeResyncSQN(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		sqnMS   uint64
		want    uint64
		wantErr bool
	}{
		{"normal", 100, 132, false},
		{"zero", 0, 32, false},
		{"near max", MaxSQN - 32, MaxSQN, false},
		{"overflow", MaxSQN, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := v.ComputeResyncSQN(tt.sqnMS)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeResyncSQN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ComputeResyncSQN() = %d, want %d", got, tt.want)
			}
		})
	}
}
