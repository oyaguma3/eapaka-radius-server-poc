package session

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/store"
)

func setupIdentifierResolver(t *testing.T, maskEnabled bool) (*miniredis.Miniredis, IdentifierResolver) {
	t.Helper()
	mr := miniredis.RunT(t)
	cfg := newTestConfig(mr.Addr())
	vc, err := store.NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	t.Cleanup(func() { vc.Close() })
	ss := store.NewSessionStore(vc)
	mgr := NewManager(ss)
	return mr, NewIdentifierResolver(mgr, maskEnabled)
}

func TestResolveIMSI_FromSession(t *testing.T) {
	mr, resolver := setupIdentifierResolver(t, true)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")
	ctx := context.Background()

	result := resolver.ResolveIMSI(ctx, "test-uuid", "", "")
	expected := "001010********9"
	if result != expected {
		t.Errorf("ResolveIMSI = %q, want %q", result, expected)
	}
}

func TestResolveIMSI_FromSessionNoMask(t *testing.T) {
	mr, resolver := setupIdentifierResolver(t, false)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")
	ctx := context.Background()

	result := resolver.ResolveIMSI(ctx, "test-uuid", "", "")
	if result != "001010123456789" {
		t.Errorf("ResolveIMSI = %q, want %q", result, "001010123456789")
	}
}

func TestResolveIMSI_FromUserName(t *testing.T) {
	_, resolver := setupIdentifierResolver(t, true)
	ctx := context.Background()

	result := resolver.ResolveIMSI(ctx, "", "0001010123456789@example.com", "")
	expected := "001010********9"
	if result != expected {
		t.Errorf("ResolveIMSI = %q, want %q", result, expected)
	}
}

func TestResolveIMSI_UserNameNotIMSI(t *testing.T) {
	_, resolver := setupIdentifierResolver(t, true)
	ctx := context.Background()

	result := resolver.ResolveIMSI(ctx, "", "user@example.com", "")
	if result != "user@example.com" {
		t.Errorf("ResolveIMSI = %q, want %q", result, "user@example.com")
	}
}

func TestResolveIMSI_FromClassUUID(t *testing.T) {
	_, resolver := setupIdentifierResolver(t, true)
	ctx := context.Background()

	result := resolver.ResolveIMSI(ctx, "", "", "550e8400-e29b-41d4-a716-446655440000")
	if result != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("ResolveIMSI = %q, want %q", result, "550e8400-e29b-41d4-a716-446655440000")
	}
}

func TestResolveIMSI_Unknown(t *testing.T) {
	_, resolver := setupIdentifierResolver(t, true)
	ctx := context.Background()

	result := resolver.ResolveIMSI(ctx, "", "", "")
	if result != "unknown" {
		t.Errorf("ResolveIMSI = %q, want %q", result, "unknown")
	}
}

func TestExtractIMSIFromIdentity(t *testing.T) {
	tests := []struct {
		name     string
		identity string
		want     string
	}{
		{name: "EAP-AKA format", identity: "0001010123456789@example.com", want: "001010123456789"},
		{name: "EAP-AKA' format", identity: "6001010123456789@example.com", want: "001010123456789"},
		{name: "raw IMSI", identity: "001010123456789", want: "001010123456789"},
		{name: "too short", identity: "12345", want: ""},
		{name: "not numeric", identity: "abcdefghijklmno", want: ""},
		{name: "non-IMSI user", identity: "user@example.com", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIMSIFromIdentity(tt.identity)
			if got != tt.want {
				t.Errorf("extractIMSIFromIdentity(%q) = %q, want %q", tt.identity, got, tt.want)
			}
		})
	}
}
