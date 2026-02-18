package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// 必須環境変数を設定
	os.Setenv("VECTOR_GATEWAY_INTERNAL_URL", "http://localhost:9090")
	defer os.Unsetenv("VECTOR_GATEWAY_INTERNAL_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.InternalURL != "http://localhost:9090" {
		t.Errorf("InternalURL = %q, want %q", cfg.InternalURL, "http://localhost:9090")
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Setenv("VECTOR_GATEWAY_INTERNAL_URL", "http://localhost:9090")
	defer os.Unsetenv("VECTOR_GATEWAY_INTERNAL_URL")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// デフォルト値の確認
	if cfg.Mode != "gateway" {
		t.Errorf("Mode = %q, want %q", cfg.Mode, "gateway")
	}
	if cfg.InternalTimeout.String() != "5s" {
		t.Errorf("InternalTimeout = %v, want 5s", cfg.InternalTimeout)
	}
	if cfg.PLMNMapRaw != "" {
		t.Errorf("PLMNMapRaw = %q, want %q", cfg.PLMNMapRaw, "")
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8080")
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "INFO")
	}
	if !cfg.LogMaskIMSI {
		t.Error("LogMaskIMSI = false, want true")
	}
	if cfg.GinMode != "release" {
		t.Errorf("GinMode = %q, want %q", cfg.GinMode, "release")
	}
}

func TestLoadMissingRequired(t *testing.T) {
	// 必須環境変数をクリア
	os.Unsetenv("VECTOR_GATEWAY_INTERNAL_URL")

	_, err := Load()
	if err == nil {
		t.Error("Load() expected error for missing required env vars")
	}
}

func TestLoadCustomValues(t *testing.T) {
	os.Setenv("VECTOR_GATEWAY_INTERNAL_URL", "http://vector-api:8080")
	os.Setenv("VECTOR_GATEWAY_MODE", "passthrough")
	os.Setenv("VECTOR_GATEWAY_INTERNAL_TIMEOUT", "10s")
	os.Setenv("VECTOR_GATEWAY_PLMN_MAP", "44010:01,44020:01")
	os.Setenv("LISTEN_ADDR", ":9090")
	os.Setenv("LOG_LEVEL", "DEBUG")
	os.Setenv("LOG_MASK_IMSI", "false")
	defer func() {
		os.Unsetenv("VECTOR_GATEWAY_INTERNAL_URL")
		os.Unsetenv("VECTOR_GATEWAY_MODE")
		os.Unsetenv("VECTOR_GATEWAY_INTERNAL_TIMEOUT")
		os.Unsetenv("VECTOR_GATEWAY_PLMN_MAP")
		os.Unsetenv("LISTEN_ADDR")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("LOG_MASK_IMSI")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Mode != "passthrough" {
		t.Errorf("Mode = %q, want %q", cfg.Mode, "passthrough")
	}
	if cfg.InternalTimeout.String() != "10s" {
		t.Errorf("InternalTimeout = %v, want 10s", cfg.InternalTimeout)
	}
	if cfg.PLMNMapRaw != "44010:01,44020:01" {
		t.Errorf("PLMNMapRaw = %q, want %q", cfg.PLMNMapRaw, "44010:01,44020:01")
	}
	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
	}
	if cfg.LogLevel != "DEBUG" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "DEBUG")
	}
	if cfg.LogMaskIMSI {
		t.Error("LogMaskIMSI = true, want false")
	}
}

func TestIsPassthrough(t *testing.T) {
	tests := []struct {
		mode string
		want bool
	}{
		{"gateway", false},
		{"passthrough", true},
		{"", false},
		{"other", false},
	}

	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			cfg := &Config{Mode: tt.mode}
			got := cfg.IsPassthrough()
			if got != tt.want {
				t.Errorf("IsPassthrough() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParsePLMNMap(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    map[string]string
		wantErr bool
	}{
		{
			"empty string",
			"",
			map[string]string{},
			false,
		},
		{
			"single entry 5 digits",
			"44010:01",
			map[string]string{"44010": "01"},
			false,
		},
		{
			"single entry 6 digits",
			"440100:01",
			map[string]string{"440100": "01"},
			false,
		},
		{
			"multiple entries",
			"44010:01,44020:02",
			map[string]string{"44010": "01", "44020": "02"},
			false,
		},
		{
			"with spaces",
			" 44010 : 01 , 44020 : 02 ",
			map[string]string{"44010": "01", "44020": "02"},
			false,
		},
		{
			"trailing comma",
			"44010:01,",
			map[string]string{"44010": "01"},
			false,
		},
		{
			"invalid format no colon",
			"44010",
			nil,
			true,
		},
		{
			"PLMN too short (4 digits)",
			"4401:01",
			nil,
			true,
		},
		{
			"PLMN too long (7 digits)",
			"4401000:01",
			nil,
			true,
		},
		{
			"PLMN non-digit",
			"4401a:01",
			nil,
			true,
		},
		{
			"BackendID wrong length",
			"44010:1",
			nil,
			true,
		},
		{
			"BackendID non-digit",
			"44010:0a",
			nil,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{PLMNMapRaw: tt.raw}
			got, err := cfg.ParsePLMNMap()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePLMNMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("ParsePLMNMap() returned %d entries, want %d", len(got), len(tt.want))
				return
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("ParsePLMNMap()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestValidatePLMN(t *testing.T) {
	tests := []struct {
		plmn    string
		wantErr bool
	}{
		{"44010", false},
		{"440100", false},
		{"4401", true},
		{"4401000", true},
		{"4401a", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.plmn, func(t *testing.T) {
			err := validatePLMN(tt.plmn)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePLMN(%q) error = %v, wantErr %v", tt.plmn, err, tt.wantErr)
			}
		})
	}
}

func TestValidateBackendID(t *testing.T) {
	tests := []struct {
		id      string
		wantErr bool
	}{
		{"00", false},
		{"01", false},
		{"99", false},
		{"0", true},
		{"001", true},
		{"0a", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			err := validateBackendID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBackendID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
			}
		})
	}
}
