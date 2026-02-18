package format

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := Bytes(tt.input); got != tt.want {
				t.Errorf("Bytes(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBytesShort(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0K"},
		{1048576, "1.0M"},
		{1073741824, "1.0G"},
		{1099511627776, "1.0T"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := BytesShort(tt.input); got != tt.want {
				t.Errorf("BytesShort(%d) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
