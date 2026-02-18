package store

import (
	"context"
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestClientStore_CRUD(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	cs := NewClientStore(client)
	ctx := context.Background()

	c := &model.RadiusClient{
		IP:     "192.168.10.1",
		Secret: "TESTSECRET123",
		Name:   "Customer01",
		Vendor: "generic",
	}

	// Create
	if err := cs.Create(ctx, c); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Create duplicate
	if err := cs.Create(ctx, c); err == nil {
		t.Error("Create() expected error for duplicate")
	}

	// Exists
	exists, err := cs.Exists(ctx, c.IP)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}

	// Get
	got, err := cs.Get(ctx, c.IP)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Secret != c.Secret {
		t.Errorf("Get().Secret = %s, want %s", got.Secret, c.Secret)
	}
	if got.Name != c.Name {
		t.Errorf("Get().Name = %s, want %s", got.Name, c.Name)
	}

	// Get not found
	_, err = cs.Get(ctx, "10.10.10.10")
	if !errors.Is(err, ErrClientNotFound) {
		t.Errorf("Get() expected ErrClientNotFound, got: %v", err)
	}

	// Update
	c.Secret = "NEWSECRET"
	if err := cs.Update(ctx, c); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ = cs.Get(ctx, c.IP)
	if got.Secret != "NEWSECRET" {
		t.Errorf("after Update, Secret = %s, want NEWSECRET", got.Secret)
	}

	// Update not found
	if err := cs.Update(ctx, &model.RadiusClient{IP: "10.10.10.10"}); !errors.Is(err, ErrClientNotFound) {
		t.Errorf("Update() expected ErrClientNotFound, got: %v", err)
	}

	// Count
	count, err := cs.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Errorf("Count() = %d, want 1", count)
	}

	// List
	list, err := cs.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List() len = %d, want 1", len(list))
	}

	// Delete
	if err := cs.Delete(ctx, c.IP); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Delete not found
	if err := cs.Delete(ctx, c.IP); !errors.Is(err, ErrClientNotFound) {
		t.Errorf("Delete() expected ErrClientNotFound, got: %v", err)
	}
}

func TestClientStore_BulkCreate(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	cs := NewClientStore(client)
	ctx := context.Background()

	// 空リスト
	if err := cs.BulkCreate(ctx, []*model.RadiusClient{}); err != nil {
		t.Fatalf("BulkCreate(empty) error = %v", err)
	}

	clients := []*model.RadiusClient{
		{IP: "192.168.1.1", Secret: "s1", Name: "AP1", Vendor: "generic"},
		{IP: "192.168.1.2", Secret: "s2", Name: "AP2", Vendor: "generic"},
	}

	if err := cs.BulkCreate(ctx, clients); err != nil {
		t.Fatalf("BulkCreate() error = %v", err)
	}

	count, _ := cs.Count(ctx)
	if count != 2 {
		t.Errorf("Count() = %d after BulkCreate, want 2", count)
	}
}

func TestClientStore_List_Empty(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	cs := NewClientStore(client)
	ctx := context.Background()

	list, err := cs.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list != nil && len(list) != 0 {
		t.Errorf("List() len = %d, want 0", len(list))
	}
}
