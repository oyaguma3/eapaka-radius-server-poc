package masterdata

import (
	"context"
	"errors"
	"testing"
	"time"

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

	// Update（SQN は書き換えない）
	sub.AMF = "8000"
	sub.SQN = "000000000020"
	if err := ss.Update(ctx, sub); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ = ss.Get(ctx, sub.IMSI)
	if got.AMF != "8000" {
		t.Errorf("after Update, AMF = %s, want 8000", got.AMF)
	}
	if got.SQN != "ff9bb4d0b607" {
		t.Errorf("after Update, SQN = %s, want ff9bb4d0b607 (unchanged)", got.SQN)
	}

	// UpdateWithSQN
	if err := ss.UpdateWithSQN(ctx, sub, "ff9bb4d0b607"); err != nil {
		t.Fatalf("UpdateWithSQN() error = %v", err)
	}

	got, _ = ss.Get(ctx, sub.IMSI)
	if got.SQN != "000000000020" {
		t.Errorf("after UpdateWithSQN, SQN = %s, want 000000000020", got.SQN)
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

func TestSubscriberStore_UpdateSQNHandling(t *testing.T) {
	const imsi = "440101234567890"
	key := SubscriberKey(imsi)

	// 編集開始時の加入者（SQN は Vector API が小文字で書き戻した値）
	register := func(t *testing.T) (*SubscriberStore, func(field string) string) {
		t.Helper()
		mr, client := newTestRedis(t)
		t.Cleanup(func() { _ = client.Close() })
		mr.HSet(key,
			"ki", "465B5CE8B199B49FAA5F0A2EE238A6BC",
			"opc", "CD63CB71954A9F4E48A5994E37A02BAF",
			"amf", "8000",
			"sqn", "00000000004a",
			"created_at", "2026-01-01T00:00:00Z",
		)
		return NewSubscriberStore(client), func(field string) string { return mr.HGet(key, field) }
	}

	edited := &model.Subscriber{
		IMSI: imsi,
		Ki:   "00112233445566778899AABBCCDDEEFF",
		OPc:  "AABBCCDDEEFF00112233445566778899",
		AMF:  "B9B9",
		SQN:  "000000000020", // 編集開始時より古い値
	}

	t.Run("Update は SQN を書き換えない", func(t *testing.T) {
		ss, hget := register(t)

		if err := ss.Update(context.Background(), edited); err != nil {
			t.Fatalf("Update() error = %v", err)
		}
		if got := hget("sqn"); got != "00000000004a" {
			t.Errorf("sqn = %s, want 00000000004a (unchanged)", got)
		}
		if hget("ki") != edited.Ki || hget("opc") != edited.OPc || hget("amf") != edited.AMF {
			t.Errorf("ki/opc/amf not updated: %s %s %s", hget("ki"), hget("opc"), hget("amf"))
		}
		if got := hget("created_at"); got != "2026-01-01T00:00:00Z" {
			t.Errorf("created_at changed: %s", got)
		}
	})

	t.Run("UpdateWithSQN は SQN が編集開始時のままなら書き換える", func(t *testing.T) {
		ss, hget := register(t)

		if err := ss.UpdateWithSQN(context.Background(), edited, "00000000004a"); err != nil {
			t.Fatalf("UpdateWithSQN() error = %v", err)
		}
		if got := hget("sqn"); got != "000000000020" {
			t.Errorf("sqn = %s, want 000000000020", got)
		}
		if got := hget("ki"); got != edited.Ki {
			t.Errorf("ki = %s, want %s", got, edited.Ki)
		}
	})

	t.Run("UpdateWithSQN は SQN が変わっていたら何も書き換えない", func(t *testing.T) {
		ss, hget := register(t)

		// 編集開始時に読んだ値（00000000003a）の後に、認証で 00000000004a に進んだ
		err := ss.UpdateWithSQN(context.Background(), edited, "00000000003a")
		if !errors.Is(err, ErrSQNChanged) {
			t.Fatalf("UpdateWithSQN() error = %v, want ErrSQNChanged", err)
		}
		if got := hget("sqn"); got != "00000000004a" {
			t.Errorf("sqn = %s, want 00000000004a (unchanged)", got)
		}
		if got := hget("ki"); got != "465B5CE8B199B49FAA5F0A2EE238A6BC" {
			t.Errorf("ki must not be updated: %s", got)
		}
	})

	t.Run("加入者がいなければキーを作らない", func(t *testing.T) {
		mr, client := newTestRedis(t)
		t.Cleanup(func() { _ = client.Close() })
		ss := NewSubscriberStore(client)

		if err := ss.Update(context.Background(), edited); !errors.Is(err, ErrSubscriberNotFound) {
			t.Errorf("Update() error = %v, want ErrSubscriberNotFound", err)
		}
		if err := ss.UpdateWithSQN(context.Background(), edited, "00000000004a"); !errors.Is(err, ErrSubscriberNotFound) {
			t.Errorf("UpdateWithSQN() error = %v, want ErrSubscriberNotFound", err)
		}
		if mr.Exists(key) {
			t.Errorf("key %s must not be created", key)
		}
	})

	t.Run("Valkey エラー", func(t *testing.T) {
		mr, client := newTestRedis(t)
		t.Cleanup(func() { _ = client.Close() })
		ss := NewSubscriberStore(client)
		mr.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()
		if err := ss.Update(ctx, edited); err == nil || errors.Is(err, ErrSubscriberNotFound) {
			t.Errorf("Update() error = %v, want connection error", err)
		}
	})
}
