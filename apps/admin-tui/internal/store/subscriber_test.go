package store

import (
	"context"
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestSubscriberStore_CRUD(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSubscriberStore(client)
	ctx := context.Background()

	sub := &model.Subscriber{
		IMSI:      "001010000000001",
		Ki:        "465b5ce8b199b49faa5f0a2ee238a6bc",
		OPc:       "cd63cb71954a9f4e48a5994e37a02baf",
		AMF:       "b9b9",
		SQN:       "ff9bb4d0b607",
		CreatedAt: "2026-01-01T00:00:00Z",
	}

	// Create
	if err := ss.Create(ctx, sub); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Create duplicate
	if err := ss.Create(ctx, sub); err == nil {
		t.Error("Create() expected error for duplicate")
	}

	// Exists
	exists, err := ss.Exists(ctx, sub.IMSI)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}

	// Get
	got, err := ss.Get(ctx, sub.IMSI)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Ki != sub.Ki {
		t.Errorf("Get().Ki = %s, want %s", got.Ki, sub.Ki)
	}
	if got.SQN != sub.SQN {
		t.Errorf("Get().SQN = %s, want %s", got.SQN, sub.SQN)
	}

	// Get not found
	_, err = ss.Get(ctx, "999999999999999")
	if !errors.Is(err, ErrSubscriberNotFound) {
		t.Errorf("Get() expected ErrSubscriberNotFound, got: %v", err)
	}

	// Update
	sub.SQN = "000000000020"
	if err := ss.Update(ctx, sub); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ = ss.Get(ctx, sub.IMSI)
	if got.SQN != "000000000020" {
		t.Errorf("after Update, SQN = %s, want 000000000020", got.SQN)
	}

	// Update not found
	if err := ss.Update(ctx, &model.Subscriber{IMSI: "999999999999999"}); !errors.Is(err, ErrSubscriberNotFound) {
		t.Errorf("Update() expected ErrSubscriberNotFound, got: %v", err)
	}

	// Count
	count, err := ss.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Errorf("Count() = %d, want 1", count)
	}

	// List
	list, err := ss.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List() len = %d, want 1", len(list))
	}

	// Delete
	if err := ss.Delete(ctx, sub.IMSI); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Delete not found
	if err := ss.Delete(ctx, sub.IMSI); !errors.Is(err, ErrSubscriberNotFound) {
		t.Errorf("Delete() expected ErrSubscriberNotFound, got: %v", err)
	}

	// Exists after delete
	exists, _ = ss.Exists(ctx, sub.IMSI)
	if exists {
		t.Error("Exists() = true after delete, want false")
	}
}

func TestSubscriberStore_BulkCreate(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSubscriberStore(client)
	ctx := context.Background()

	// 空リスト
	if err := ss.BulkCreate(ctx, []*model.Subscriber{}); err != nil {
		t.Fatalf("BulkCreate(empty) error = %v", err)
	}

	subs := []*model.Subscriber{
		{IMSI: "001010000000001", Ki: "ki1", OPc: "opc1", AMF: "amf1", SQN: "sqn1"},
		{IMSI: "001010000000002", Ki: "ki2", OPc: "opc2", AMF: "amf2", SQN: "sqn2"},
	}

	if err := ss.BulkCreate(ctx, subs); err != nil {
		t.Fatalf("BulkCreate() error = %v", err)
	}

	count, _ := ss.Count(ctx)
	if count != 2 {
		t.Errorf("Count() = %d after BulkCreate, want 2", count)
	}
}

func TestSubscriberStore_Create_DefaultCreatedAt(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSubscriberStore(client)
	ctx := context.Background()

	// CreatedAtが空の場合、自動設定される
	sub := &model.Subscriber{
		IMSI: "001010000000003",
		Ki:   "ki", OPc: "opc", AMF: "amf", SQN: "sqn",
	}
	if err := ss.Create(ctx, sub); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	got, _ := ss.Get(ctx, sub.IMSI)
	if got.CreatedAt == "" {
		t.Error("CreatedAt should be auto-set when empty")
	}
}

func TestSubscriberStore_List_Empty(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSubscriberStore(client)
	ctx := context.Background()

	list, err := ss.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if list != nil && len(list) != 0 {
		t.Errorf("List() len = %d, want 0", len(list))
	}
}
