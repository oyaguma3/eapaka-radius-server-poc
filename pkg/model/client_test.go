package model

import "testing"

func TestNewRadiusClient(t *testing.T) {
	client := NewRadiusClient(
		"192.168.1.100",
		"shared-secret-123",
		"ap-office-01",
		"Cisco",
	)

	if client.IP != "192.168.1.100" {
		t.Errorf("IP = %q, want %q", client.IP, "192.168.1.100")
	}
	if client.Secret != "shared-secret-123" {
		t.Errorf("Secret = %q, want %q", client.Secret, "shared-secret-123")
	}
	if client.Name != "ap-office-01" {
		t.Errorf("Name = %q, want %q", client.Name, "ap-office-01")
	}
	if client.Vendor != "Cisco" {
		t.Errorf("Vendor = %q, want %q", client.Vendor, "Cisco")
	}
}

func TestRadiusClientStruct(t *testing.T) {
	client := RadiusClient{
		IP:     "10.0.0.1",
		Secret: "test-secret",
		Name:   "test-ap",
		Vendor: "",
	}

	if client.IP != "10.0.0.1" {
		t.Errorf("IP = %q, want %q", client.IP, "10.0.0.1")
	}
	if client.Vendor != "" {
		t.Errorf("Vendor = %q, want empty", client.Vendor)
	}
}
