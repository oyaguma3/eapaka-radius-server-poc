package store

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// newTestRedis はテスト用のminiredisインスタンスとRedisクライアントを返す。
func newTestRedis(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	return mr, client
}

func TestNew(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	s := New(client)
	if s == nil {
		t.Fatal("expected non-nil Store")
	}
}

func TestStore_Client(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	s := New(client)
	if s.Client() != client {
		t.Error("Client() should return the same redis client")
	}
}

func TestStore_Ping(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	s := New(client)
	if err := s.Ping(context.Background()); err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestStore_Close(t *testing.T) {
	_, client := newTestRedis(t)

	s := New(client)
	if err := s.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}
