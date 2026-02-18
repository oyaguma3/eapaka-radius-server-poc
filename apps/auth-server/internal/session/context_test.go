package session

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/store"
)

// テスト用ValkeyClientを生成するヘルパー
func newTestValkeyClient(t *testing.T, mr *miniredis.Miniredis) *store.ValkeyClient {
	t.Helper()
	cfg := &config.Config{
		RedisHost:    mr.Host(),
		RedisPort:    mr.Port(),
		RedisPass:    "",
		VectorAPIURL: "http://localhost:8080",
		NetworkName:  "WLAN",
	}
	vc, err := store.NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	t.Cleanup(func() { vc.Close() })
	return vc
}

func TestContextStoreCreate(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	eapCtx := &EAPContext{
		IMSI:                 "440101234567890",
		Stage:                "identity",
		EAPType:              23,
		RAND:                 "aabbccdd",
		AUTN:                 "11223344",
		XRES:                 "deadbeef",
		Kaut:                 "cafebabe",
		MSK:                  "01020304",
		ResyncCount:          0,
		PermanentIDRequested: false,
	}

	if err := cs.Create(ctx, "trace-001", eapCtx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Valkeyに保存されたデータの確認
	val := mr.HGet("eap:trace-001", "imsi")
	if val != "440101234567890" {
		t.Errorf("imsi: got %v, want 440101234567890", val)
	}
}

func TestContextStoreCreateTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	eapCtx := &EAPContext{IMSI: "440101234567890", Stage: "identity"}
	if err := cs.Create(ctx, "trace-ttl", eapCtx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	ttl := mr.TTL("eap:trace-ttl")
	if ttl != config.EAPContextTTL {
		t.Errorf("TTL: got %v, want %v", ttl, config.EAPContextTTL)
	}
}

func TestContextStoreGet(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	// テストデータ投入
	mr.HSet("eap:trace-get", "imsi", "440101234567890")
	mr.HSet("eap:trace-get", "stage", "challenge")
	mr.HSet("eap:trace-get", "eap_type", "50")
	mr.HSet("eap:trace-get", "rand", "aabb")
	mr.HSet("eap:trace-get", "autn", "ccdd")
	mr.HSet("eap:trace-get", "xres", "eeff")
	mr.HSet("eap:trace-get", "k_aut", "1122")
	mr.HSet("eap:trace-get", "msk", "3344")
	mr.HSet("eap:trace-get", "resync_count", "2")
	mr.HSet("eap:trace-get", "permanent_id_requested", "1")

	got, err := cs.Get(ctx, "trace-get")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.IMSI != "440101234567890" {
		t.Errorf("IMSI: got %v, want 440101234567890", got.IMSI)
	}
	if got.Stage != "challenge" {
		t.Errorf("Stage: got %v, want challenge", got.Stage)
	}
	if got.EAPType != 50 {
		t.Errorf("EAPType: got %v, want 50", got.EAPType)
	}
	if got.RAND != "aabb" {
		t.Errorf("RAND: got %v, want aabb", got.RAND)
	}
	if got.AUTN != "ccdd" {
		t.Errorf("AUTN: got %v, want ccdd", got.AUTN)
	}
	if got.XRES != "eeff" {
		t.Errorf("XRES: got %v, want eeff", got.XRES)
	}
	if got.Kaut != "1122" {
		t.Errorf("Kaut: got %v, want 1122", got.Kaut)
	}
	if got.MSK != "3344" {
		t.Errorf("MSK: got %v, want 3344", got.MSK)
	}
	if got.ResyncCount != 2 {
		t.Errorf("ResyncCount: got %v, want 2", got.ResyncCount)
	}
	if got.PermanentIDRequested != true {
		t.Errorf("PermanentIDRequested: got %v, want true", got.PermanentIDRequested)
	}
}

func TestContextStoreGetNotFound(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	_, err := cs.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrContextNotFound) {
		t.Errorf("expected ErrContextNotFound, got: %v", err)
	}
}

func TestContextStoreUpdate(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	// 初期データ作成
	eapCtx := &EAPContext{IMSI: "440101234567890", Stage: "identity", EAPType: 23}
	if err := cs.Create(ctx, "trace-upd", eapCtx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 部分更新
	updates := map[string]any{
		"stage":        "challenge",
		"resync_count": 1,
	}
	if err := cs.Update(ctx, "trace-upd", updates); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// 更新確認
	got, err := cs.Get(ctx, "trace-upd")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.Stage != "challenge" {
		t.Errorf("Stage: got %v, want challenge", got.Stage)
	}
	if got.ResyncCount != 1 {
		t.Errorf("ResyncCount: got %v, want 1", got.ResyncCount)
	}
	// 変更していないフィールドは保持される
	if got.IMSI != "440101234567890" {
		t.Errorf("IMSI: got %v, want 440101234567890 (unchanged)", got.IMSI)
	}
}

func TestContextStoreUpdateNotFound(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	err := cs.Update(ctx, "nonexistent", map[string]any{"stage": "challenge"})
	if !errors.Is(err, ErrContextNotFound) {
		t.Errorf("expected ErrContextNotFound, got: %v", err)
	}
}

func TestContextStoreUpdateRefreshTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	eapCtx := &EAPContext{IMSI: "440101234567890", Stage: "identity"}
	if err := cs.Create(ctx, "trace-ttl2", eapCtx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// 時間を進める
	mr.FastForward(30 * time.Second)

	// Update実行でTTLがリフレッシュされるか
	if err := cs.Update(ctx, "trace-ttl2", map[string]any{"stage": "challenge"}); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	ttl := mr.TTL("eap:trace-ttl2")
	if ttl != config.EAPContextTTL {
		t.Errorf("TTL after update: got %v, want %v", ttl, config.EAPContextTTL)
	}
}

func TestContextStoreDelete(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	eapCtx := &EAPContext{IMSI: "440101234567890", Stage: "identity"}
	if err := cs.Create(ctx, "trace-del", eapCtx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if err := cs.Delete(ctx, "trace-del"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	exists, _ := cs.Exists(ctx, "trace-del")
	if exists {
		t.Error("key should not exist after delete")
	}
}

func TestContextStoreDeleteNonExistent(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	// 存在しないキーの削除はエラーにならない
	if err := cs.Delete(ctx, "nonexistent"); err != nil {
		t.Errorf("Delete of non-existent key should not error, got: %v", err)
	}
}

func TestContextStoreExists(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	eapCtx := &EAPContext{IMSI: "440101234567890", Stage: "identity"}
	if err := cs.Create(ctx, "trace-exists", eapCtx); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	exists, err := cs.Exists(ctx, "trace-exists")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected key to exist")
	}
}

func TestContextStoreExistsNotFound(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	exists, err := cs.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("expected key to not exist")
	}
}

func TestContextStoreValkeyError(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	cs := NewContextStore(vc)
	ctx := context.Background()

	// Valkey停止
	mr.Close()

	_, err := cs.Get(ctx, "any-key")
	if err == nil {
		t.Fatal("expected error when Valkey is down")
	}
	if !errors.Is(err, store.ErrValkeyUnavailable) {
		t.Errorf("expected ErrValkeyUnavailable, got: %v", err)
	}
}
