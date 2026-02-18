package policy

import (
	"crypto/rand"
	"testing"
)

func TestGenerateMPPEKeys(t *testing.T) {
	msk := make([]byte, 64)
	if _, err := rand.Read(msk); err != nil {
		t.Fatalf("failed to generate random MSK: %v", err)
	}
	secret := []byte("testing123")
	reqAuth := make([]byte, 16)
	if _, err := rand.Read(reqAuth); err != nil {
		t.Fatalf("failed to generate random reqAuth: %v", err)
	}

	recvKey, sendKey, err := GenerateMPPEKeys(msk, secret, reqAuth)
	if err != nil {
		t.Fatalf("GenerateMPPEKeys failed: %v", err)
	}
	if recvKey == nil {
		t.Error("recvKey is nil")
	}
	if sendKey == nil {
		t.Error("sendKey is nil")
	}
	if len(recvKey) == 0 {
		t.Error("recvKey is empty")
	}
	if len(sendKey) == 0 {
		t.Error("sendKey is empty")
	}
}

func TestGenerateMPPEKeysMSKTooShort(t *testing.T) {
	msk := make([]byte, 32) // 64バイト未満
	secret := []byte("testing123")
	reqAuth := make([]byte, 16)

	_, _, err := GenerateMPPEKeys(msk, secret, reqAuth)
	if err == nil {
		t.Fatal("expected error for short MSK, got nil")
	}
}

func TestVlanAVPs(t *testing.T) {
	avps := VlanAVPs("100")
	if avps == nil {
		t.Fatal("expected non-nil AVPs")
	}
	if avps["Tunnel-Type"] != 13 {
		t.Errorf("Tunnel-Type = %v, want 13", avps["Tunnel-Type"])
	}
	if avps["Tunnel-Medium-Type"] != 6 {
		t.Errorf("Tunnel-Medium-Type = %v, want 6", avps["Tunnel-Medium-Type"])
	}
	if avps["Tunnel-Private-Group-Id"] != "100" {
		t.Errorf("Tunnel-Private-Group-Id = %v, want %q", avps["Tunnel-Private-Group-Id"], "100")
	}
}

func TestVlanAVPsEmpty(t *testing.T) {
	avps := VlanAVPs("")
	if avps != nil {
		t.Errorf("expected nil for empty vlanID, got %v", avps)
	}
}

func TestSessionTimeoutValue(t *testing.T) {
	val, ok := SessionTimeoutValue(3600)
	if !ok {
		t.Fatal("expected ok=true for positive timeout")
	}
	if val != 3600 {
		t.Errorf("SessionTimeoutValue = %d, want 3600", val)
	}
}

func TestSessionTimeoutValueZero(t *testing.T) {
	val, ok := SessionTimeoutValue(0)
	if ok {
		t.Fatal("expected ok=false for zero timeout")
	}
	if val != 0 {
		t.Errorf("SessionTimeoutValue = %d, want 0", val)
	}

	// 負の値もfalse
	val, ok = SessionTimeoutValue(-1)
	if ok {
		t.Fatal("expected ok=false for negative timeout")
	}
	if val != 0 {
		t.Errorf("SessionTimeoutValue = %d, want 0", val)
	}
}
