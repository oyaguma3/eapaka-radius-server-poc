package store

import "testing"

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
