package server

import (
	"testing"

	"layeh.com/radius"
)

func TestNewServer(t *testing.T) {
	handler := radius.HandlerFunc(func(w radius.ResponseWriter, r *radius.Request) {})
	secretSource := radius.StaticSecretSource([]byte("test-secret"))

	s := NewServer(":1812", handler, secretSource)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.ps == nil {
		t.Fatal("PacketServer is nil")
	}
	if s.ps.Addr != ":1812" {
		t.Errorf("Addr: got %q, want %q", s.ps.Addr, ":1812")
	}
}

func TestNewServer_CustomAddr(t *testing.T) {
	handler := radius.HandlerFunc(func(w radius.ResponseWriter, r *radius.Request) {})
	secretSource := radius.StaticSecretSource([]byte("secret"))

	s := NewServer(":1813", handler, secretSource)
	if s.ps.Addr != ":1813" {
		t.Errorf("Addr: got %q, want %q", s.ps.Addr, ":1813")
	}
}
