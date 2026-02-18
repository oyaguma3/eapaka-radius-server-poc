package store

import "testing"

func TestSubscriberKey(t *testing.T) {
	key := SubscriberKey("440101234567890")
	expected := "sub:440101234567890"
	if key != expected {
		t.Errorf("SubscriberKey() = %s, want %s", key, expected)
	}
}

func TestClientKey(t *testing.T) {
	key := ClientKey("192.168.1.1")
	expected := "client:192.168.1.1"
	if key != expected {
		t.Errorf("ClientKey() = %s, want %s", key, expected)
	}
}

func TestPolicyKey(t *testing.T) {
	key := PolicyKey("440101234567890")
	expected := "policy:440101234567890"
	if key != expected {
		t.Errorf("PolicyKey() = %s, want %s", key, expected)
	}
}

func TestSessionKey(t *testing.T) {
	key := SessionKey("abc-123-def")
	expected := "sess:abc-123-def"
	if key != expected {
		t.Errorf("SessionKey() = %s, want %s", key, expected)
	}
}

func TestUserIndexKey(t *testing.T) {
	key := UserIndexKey("440101234567890")
	expected := "idx:user:440101234567890"
	if key != expected {
		t.Errorf("UserIndexKey() = %s, want %s", key, expected)
	}
}
