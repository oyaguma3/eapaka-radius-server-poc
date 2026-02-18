package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/policy"
)

func TestGetPolicy(t *testing.T) {
	mr := miniredis.RunT(t)

	rules := []policy.PolicyRule{
		{
			NasID:          "nas-01",
			AllowedSSIDs:   []string{"SSID-A", "SSID-B"},
			VlanID:         "100",
			SessionTimeout: 3600,
		},
	}
	rulesJSON, _ := json.Marshal(rules)
	mr.HSet("policy:001010123456789", "default", "allow")
	mr.HSet("policy:001010123456789", "rules", string(rulesJSON))

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ps := NewPolicyStore(vc)
	ctx := context.Background()

	p, err := ps.GetPolicy(ctx, "001010123456789")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if p.Default != "allow" {
		t.Errorf("Default = %q, want %q", p.Default, "allow")
	}
	if len(p.Rules) != 1 {
		t.Fatalf("Rules length = %d, want 1", len(p.Rules))
	}
	if p.Rules[0].NasID != "nas-01" {
		t.Errorf("Rules[0].NasID = %q, want %q", p.Rules[0].NasID, "nas-01")
	}
	if p.Rules[0].VlanID != "100" {
		t.Errorf("Rules[0].VlanID = %q, want %q", p.Rules[0].VlanID, "100")
	}
	if p.Rules[0].SessionTimeout != 3600 {
		t.Errorf("Rules[0].SessionTimeout = %d, want 3600", p.Rules[0].SessionTimeout)
	}
	if len(p.Rules[0].AllowedSSIDs) != 2 {
		t.Errorf("Rules[0].AllowedSSIDs length = %d, want 2", len(p.Rules[0].AllowedSSIDs))
	}
}

func TestGetPolicyNotFound(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ps := NewPolicyStore(vc)
	ctx := context.Background()

	_, err = ps.GetPolicy(ctx, "999999999999999")
	if err == nil {
		t.Fatal("expected error for missing policy, got nil")
	}
	if !errors.Is(err, policy.ErrPolicyNotFound) {
		t.Errorf("expected ErrPolicyNotFound, got: %v", err)
	}
}

func TestGetPolicyInvalidJSON(t *testing.T) {
	mr := miniredis.RunT(t)

	mr.HSet("policy:001010123456789", "default", "allow")
	mr.HSet("policy:001010123456789", "rules", "invalid-json{{{")

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ps := NewPolicyStore(vc)
	ctx := context.Background()

	_, err = ps.GetPolicy(ctx, "001010123456789")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !errors.Is(err, policy.ErrPolicyInvalid) {
		t.Errorf("expected ErrPolicyInvalid, got: %v", err)
	}
}

func TestGetPolicyInvalidDefault(t *testing.T) {
	mr := miniredis.RunT(t)

	rules := []policy.PolicyRule{}
	rulesJSON, _ := json.Marshal(rules)
	mr.HSet("policy:001010123456789", "default", "unknown")
	mr.HSet("policy:001010123456789", "rules", string(rulesJSON))

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ps := NewPolicyStore(vc)
	ctx := context.Background()

	p, err := ps.GetPolicy(ctx, "001010123456789")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	// 不正なdefault値は"deny"扱い
	if p.Default != "deny" {
		t.Errorf("Default = %q, want %q (invalid default should be treated as deny)", p.Default, "deny")
	}
}

func TestGetPolicyEmptyRules(t *testing.T) {
	mr := miniredis.RunT(t)

	mr.HSet("policy:001010123456789", "default", "deny")
	// rulesフィールドなし

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ps := NewPolicyStore(vc)
	ctx := context.Background()

	p, err := ps.GetPolicy(ctx, "001010123456789")
	if err != nil {
		t.Fatalf("GetPolicy failed: %v", err)
	}
	if len(p.Rules) != 0 {
		t.Errorf("Rules length = %d, want 0", len(p.Rules))
	}
}

func TestGetPolicyValkeyError(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}

	// Valkey停止
	mr.Close()

	ps := NewPolicyStore(vc)
	ctx := context.Background()

	_, err = ps.GetPolicy(ctx, "001010123456789")
	if err == nil {
		t.Fatal("expected error when Valkey is down, got nil")
	}
	if !errors.Is(err, ErrValkeyUnavailable) {
		t.Errorf("expected ErrValkeyUnavailable, got: %v", err)
	}
}
