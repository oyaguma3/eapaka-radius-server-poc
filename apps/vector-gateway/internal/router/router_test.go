package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/backend"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
)

// newTestRegistry はテスト用のRegistryを生成する。
func newTestRegistry() *backend.Registry {
	cfg := &config.Config{
		InternalURL:     "http://localhost:8080",
		InternalTimeout: 5 * time.Second,
	}
	return backend.NewRegistry(cfg)
}

func TestSelectBackend_Passthrough(t *testing.T) {
	plmnMap := map[string]string{"44010": "99"}
	r := NewRouter(plmnMap, newTestRegistry(), true)

	// passthroughモードでは常にデフォルト
	b, err := r.SelectBackend("440101234567890")
	if err != nil {
		t.Fatalf("SelectBackend() error = %v", err)
	}
	if b.ID() != "00" {
		t.Errorf("ID() = %q, want %q (default)", b.ID(), "00")
	}
}

func TestSelectBackend_EmptyPLMNMap(t *testing.T) {
	r := NewRouter(map[string]string{}, newTestRegistry(), false)

	b, err := r.SelectBackend("440101234567890")
	if err != nil {
		t.Fatalf("SelectBackend() error = %v", err)
	}
	if b.ID() != "00" {
		t.Errorf("ID() = %q, want %q (default)", b.ID(), "00")
	}
}

func TestSelectBackend_5DigitPLMNMatch(t *testing.T) {
	// 5桁PLMNで内部バックエンド(00)にマッチ
	plmnMap := map[string]string{"44010": "00"}
	r := NewRouter(plmnMap, newTestRegistry(), false)

	b, err := r.SelectBackend("440101234567890")
	if err != nil {
		t.Fatalf("SelectBackend() error = %v", err)
	}
	if b.ID() != "00" {
		t.Errorf("ID() = %q, want %q", b.ID(), "00")
	}
}

func TestSelectBackend_6DigitPLMNMatch(t *testing.T) {
	// 6桁PLMNが5桁より優先される
	plmnMap := map[string]string{
		"440101": "00",
		"44010":  "00",
	}
	r := NewRouter(plmnMap, newTestRegistry(), false)

	b, err := r.SelectBackend("440101234567890")
	if err != nil {
		t.Fatalf("SelectBackend() error = %v", err)
	}
	// 6桁優先でマッチ
	if b.ID() != "00" {
		t.Errorf("ID() = %q, want %q", b.ID(), "00")
	}
}

func TestSelectBackend_NoMatch_ReturnsDefault(t *testing.T) {
	// マッチしないPLMNの場合はデフォルト
	plmnMap := map[string]string{"99999": "00"}
	r := NewRouter(plmnMap, newTestRegistry(), false)

	b, err := r.SelectBackend("440101234567890")
	if err != nil {
		t.Fatalf("SelectBackend() error = %v", err)
	}
	if b.ID() != "00" {
		t.Errorf("ID() = %q, want %q (default)", b.ID(), "00")
	}
}

func TestSelectBackend_NotImplementedBackend(t *testing.T) {
	// 未実装バックエンドIDにマッチした場合
	plmnMap := map[string]string{"44010": "99"}
	r := NewRouter(plmnMap, newTestRegistry(), false)

	_, err := r.SelectBackend("440101234567890")
	if err == nil {
		t.Fatal("SelectBackend() expected error for not implemented backend")
	}

	var notImpl *backend.BackendNotImplementedError
	if !errors.As(err, &notImpl) {
		t.Fatalf("expected BackendNotImplementedError, got %T", err)
	}
	if notImpl.ID != "99" {
		t.Errorf("ID = %q, want %q", notImpl.ID, "99")
	}
}

func TestExtractPLMNs(t *testing.T) {
	tests := []struct {
		name string
		imsi string
		want []string
	}{
		{"15 digit IMSI", "440101234567890", []string{"440101", "44010"}},
		{"6 digit string", "440101", []string{"440101", "44010"}},
		{"5 digit string", "44010", []string{"44010"}},
		{"4 digit string", "4401", []string{}},
		{"empty string", "", []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPLMNs(tt.imsi)
			if len(got) != len(tt.want) {
				t.Fatalf("extractPLMNs(%q) returned %d candidates, want %d", tt.imsi, len(got), len(tt.want))
			}
			for i, v := range tt.want {
				if got[i] != v {
					t.Errorf("extractPLMNs(%q)[%d] = %q, want %q", tt.imsi, i, got[i], v)
				}
			}
		})
	}
}

// mockBackend はテスト用のバックエンドモック。
type mockBackend struct {
	id       string
	name     string
	response *backend.VectorResponse
	err      error
}

func (m *mockBackend) GetVector(ctx context.Context, req *backend.VectorRequest) (*backend.VectorResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockBackend) ID() string   { return m.id }
func (m *mockBackend) Name() string { return m.name }
