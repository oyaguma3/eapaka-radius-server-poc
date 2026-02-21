package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/redis/go-redis/v9"
)

// setSessionHash はテスト用にAuth/Acctサーバーと同じ形式でセッションをRedis Hashに保存する。
func setSessionHash(ctx context.Context, client *redis.Client, uuid, imsi, nasIP, clientIP, acctID string, startTime, inputOctets, outputOctets int64) {
	key := SessionKey(uuid)
	fields := map[string]any{
		"imsi":          imsi,
		"nas_ip":        nasIP,
		"client_ip":     clientIP,
		"acct_id":       acctID,
		"start_time":    fmt.Sprintf("%d", startTime),
		"input_octets":  fmt.Sprintf("%d", inputOctets),
		"output_octets": fmt.Sprintf("%d", outputOctets),
	}
	client.HSet(ctx, key, fields)
}

func TestSessionStore_Get(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSessionStore(client)
	ctx := context.Background()

	// Hash形式で保存（Auth/Acctサーバーと同じ）
	setSessionHash(ctx, client, "test-uuid-001", "001010000000001", "192.168.10.1", "10.0.0.1", "acct-001", 1700000000, 0, 0)

	// Get
	got, err := ss.Get(ctx, "test-uuid-001")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.UUID != "test-uuid-001" {
		t.Errorf("Get().UUID = %s, want test-uuid-001", got.UUID)
	}
	if got.IMSI != "001010000000001" {
		t.Errorf("Get().IMSI = %s, want 001010000000001", got.IMSI)
	}
	if got.NasIP != "192.168.10.1" {
		t.Errorf("Get().NasIP = %s, want 192.168.10.1", got.NasIP)
	}
	if got.ClientIP != "10.0.0.1" {
		t.Errorf("Get().ClientIP = %s, want 10.0.0.1", got.ClientIP)
	}
	if got.AcctSessionID != "acct-001" {
		t.Errorf("Get().AcctSessionID = %s, want acct-001", got.AcctSessionID)
	}
	if got.StartTime != 1700000000 {
		t.Errorf("Get().StartTime = %d, want 1700000000", got.StartTime)
	}

	// Get not found
	_, err = ss.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Get() expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionStore_List(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSessionStore(client)
	ctx := context.Background()

	// 空リスト
	list, err := ss.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List() len = %d, want 0", len(list))
	}

	// セッション追加（Hash形式）
	setSessionHash(ctx, client, "uuid-1", "001010000000001", "192.168.10.1", "", "", 0, 0, 0)
	setSessionHash(ctx, client, "uuid-2", "001010000000002", "192.168.10.2", "", "", 0, 0, 0)

	list, err = ss.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List() len = %d, want 2", len(list))
	}
}

func TestSessionStore_Count(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSessionStore(client)
	ctx := context.Background()

	count, err := ss.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Count() = %d, want 0", count)
	}

	// セッション追加（Hash形式）
	setSessionHash(ctx, client, "uuid-1", "001010000000001", "", "", "", 0, 0, 0)

	count, _ = ss.Count(ctx)
	if count != 1 {
		t.Errorf("Count() = %d, want 1", count)
	}
}

func TestSessionStore_GetByIMSI(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSessionStore(client)
	ctx := context.Background()

	imsi := "001010000000001"

	// 空の場合
	sessions, err := ss.GetByIMSI(ctx, imsi)
	if err != nil {
		t.Fatalf("GetByIMSI() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("GetByIMSI() len = %d, want 0", len(sessions))
	}

	// セッションとインデックスを追加（Hash形式）
	setSessionHash(ctx, client, "uuid-1", imsi, "192.168.10.1", "", "", 0, 0, 0)
	client.SAdd(ctx, UserIndexKey(imsi), "uuid-1")

	sessions, err = ss.GetByIMSI(ctx, imsi)
	if err != nil {
		t.Fatalf("GetByIMSI() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("GetByIMSI() len = %d, want 1", len(sessions))
	}
	if sessions[0].NasIP != "192.168.10.1" {
		t.Errorf("GetByIMSI()[0].NasIP = %s, want 192.168.10.1", sessions[0].NasIP)
	}
}

func TestSessionStore_GetByIMSI_StaleCleanup(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSessionStore(client)
	ctx := context.Background()

	imsi := "001010000000001"

	// インデックスに存在するが実セッションは存在しないUUID（stale）
	client.SAdd(ctx, UserIndexKey(imsi), "stale-uuid")

	sessions, err := ss.GetByIMSI(ctx, imsi)
	if err != nil {
		t.Fatalf("GetByIMSI() error = %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("GetByIMSI() len = %d, want 0", len(sessions))
	}

	// stale UUIDがインデックスからクリーンアップされたか確認
	members, _ := client.SMembers(ctx, UserIndexKey(imsi)).Result()
	if len(members) != 0 {
		t.Errorf("stale UUID should be cleaned up, remaining members: %v", members)
	}
}

func TestSessionStore_GetByIMSI_ScanFallback(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSessionStore(client)
	ctx := context.Background()

	imsi := "001010000000001"

	// セッションHashは存在するが idx:user インデックスは作成しない
	setSessionHash(ctx, client, "uuid-fb-1", imsi, "192.168.10.1", "10.0.0.1", "acct-fb-1", 1700000000, 100, 200)
	setSessionHash(ctx, client, "uuid-fb-2", "001010000000099", "192.168.10.2", "10.0.0.2", "acct-fb-2", 1700000000, 0, 0)

	// インデックスなしでも SCAN フォールバックで検索できること
	sessions, err := ss.GetByIMSI(ctx, imsi)
	if err != nil {
		t.Fatalf("GetByIMSI() with scan fallback error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("GetByIMSI() with scan fallback len = %d, want 1", len(sessions))
	}
	if len(sessions) > 0 && sessions[0].UUID != "uuid-fb-1" {
		t.Errorf("GetByIMSI() with scan fallback UUID = %s, want uuid-fb-1", sessions[0].UUID)
	}
}

func TestSessionStore_GetSessionCount(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()
	_ = mr

	ss := NewSessionStore(client)
	ctx := context.Background()

	imsi := "001010000000001"

	count, err := ss.GetSessionCount(ctx, imsi)
	if err != nil {
		t.Fatalf("GetSessionCount() error = %v", err)
	}
	if count != 0 {
		t.Errorf("GetSessionCount() = %d, want 0", count)
	}

	client.SAdd(ctx, UserIndexKey(imsi), "uuid-1", "uuid-2")

	count, _ = ss.GetSessionCount(ctx, imsi)
	if count != 2 {
		t.Errorf("GetSessionCount() = %d, want 2", count)
	}
}

func TestSessionStore_List_WithInvalidEntry(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSessionStore(client)
	ctx := context.Background()

	// 正常なセッション（Hash形式）
	setSessionHash(ctx, client, "uuid-1", "001010000000001", "", "", "", 0, 0, 0)

	// 不正なstart_timeを持つセッション
	client.HSet(ctx, SessionKey("uuid-2"), map[string]any{
		"imsi":       "001010000000002",
		"start_time": "not-a-number",
	})

	list, err := ss.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	// 不正データはスキップされるので1件のみ
	if len(list) != 1 {
		t.Errorf("List() len = %d, want 1", len(list))
	}
}

// redisパッケージのインポートを使用していることを保証
var _ = redis.Nil
