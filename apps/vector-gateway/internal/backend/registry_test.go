package backend

import (
	"errors"
	"testing"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
)

func TestNewRegistry(t *testing.T) {
	cfg := &config.Config{
		InternalURL:     "http://localhost:8080",
		InternalTimeout: 5 * time.Second,
	}

	r := NewRegistry(cfg)

	// デフォルトバックエンドが登録されていることを確認
	defaultBackend := r.Default()
	if defaultBackend == nil {
		t.Fatal("Default() returned nil")
	}
	if defaultBackend.ID() != "00" {
		t.Errorf("Default().ID() = %q, want %q", defaultBackend.ID(), "00")
	}
}

func TestRegistryGet(t *testing.T) {
	cfg := &config.Config{
		InternalURL:     "http://localhost:8080",
		InternalTimeout: 5 * time.Second,
	}
	r := NewRegistry(cfg)

	t.Run("existing backend", func(t *testing.T) {
		b, err := r.Get("00")
		if err != nil {
			t.Fatalf("Get(\"00\") error = %v", err)
		}
		if b.ID() != "00" {
			t.Errorf("ID() = %q, want %q", b.ID(), "00")
		}
	})

	t.Run("non-existing backend", func(t *testing.T) {
		_, err := r.Get("99")
		if err == nil {
			t.Fatal("Get(\"99\") expected error")
		}

		var notImpl *BackendNotImplementedError
		if !errors.As(err, &notImpl) {
			t.Fatalf("expected BackendNotImplementedError, got %T", err)
		}
		if notImpl.ID != "99" {
			t.Errorf("ID = %q, want %q", notImpl.ID, "99")
		}
	})
}
