package masterdata

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestRedis はテスト用のminiredisとクライアントを用意する。
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return mr, client
}

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
