package validation

import "testing"

func TestValidateIPv4(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"192.168.1.1", false},
		{"0.0.0.0", false},
		{"255.255.255.255", false},
		{"10.0.0.1", false},
		{"", true},
		{"256.1.1.1", true},
		{"192.168.1", true},
		{"192.168.1.1.1", true},
		{"192.168.1.a", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateIPv4(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIPv4(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateSecret(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"secret123", false},
		{"!@#$%^&*()", false},
		{"a", false},
		{"", true},
		{" ", true},
		{"secret with space", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateSecret(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSecret(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateClientName(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"ap-01", false},
		{"AP_01", false},
		{"client123", false},
		{"a", false},
		{"", true},
		{"client with space", true},
		{"client@123", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateClientName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateClientName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateVendor(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"Cisco", false},
		{"HP-Aruba", false},
		{"Vendor Name", false},
		{"", false},
		{"Vendor@123", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := ValidateVendor(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVendor(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestValidateClient(t *testing.T) {
	t.Run("valid input", func(t *testing.T) {
		input := &ClientInput{
			IP:     "192.168.1.1",
			Secret: "secret123",
			Name:   "ap-01",
			Vendor: "Cisco",
		}
		errs := ValidateClient(input)
		if len(errs) != 0 {
			t.Errorf("expected no errors, got %v", errs)
		}
	})

	t.Run("invalid input", func(t *testing.T) {
		input := &ClientInput{
			IP:     "",
			Secret: "",
			Name:   "",
			Vendor: "",
		}
		errs := ValidateClient(input)
		// IP, Secret, Name are required; Vendor is optional
		if len(errs) != 3 {
			t.Errorf("expected 3 errors, got %d", len(errs))
		}
	})
}

func TestNormalizeClientInput(t *testing.T) {
	input := &ClientInput{
		IP:     "  192.168.1.1  ",
		Secret: "  secret123  ",
		Name:   "  ap-01  ",
		Vendor: "  Cisco  ",
	}

	normalized := NormalizeClientInput(input)

	if normalized.IP != "192.168.1.1" {
		t.Errorf("expected IP '192.168.1.1', got '%s'", normalized.IP)
	}
	if normalized.Secret != "secret123" {
		t.Errorf("expected Secret 'secret123', got '%s'", normalized.Secret)
	}
	if normalized.Name != "ap-01" {
		t.Errorf("expected Name 'ap-01', got '%s'", normalized.Name)
	}
	if normalized.Vendor != "Cisco" {
		t.Errorf("expected Vendor 'Cisco', got '%s'", normalized.Vendor)
	}
}
