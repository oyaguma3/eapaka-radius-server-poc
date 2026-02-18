package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

func TestSessionStore_Get(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSessionStore(client)
	ctx := context.Background()

	session := &model.Session{
		UUID:          "test-uuid-001",
		IMSI:          "001010000000001",
		NasIP:         "192.168.10.1",
		ClientIP:      "10.0.0.1",
		AcctSessionID: "acct-001",
		StartTime:     1700000000,
	}

	// JSONで保存
	data, _ := json.Marshal(session)
	client.Set(ctx, SessionKey(session.UUID), data, 0)

	// Get
	got, err := ss.Get(ctx, session.UUID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.IMSI != session.IMSI {
		t.Errorf("Get().IMSI = %s, want %s", got.IMSI, session.IMSI)
	}
	if got.NasIP != session.NasIP {
		t.Errorf("Get().NasIP = %s, want %s", got.NasIP, session.NasIP)
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

	// セッション追加
	sessions := []model.Session{
		{UUID: "uuid-1", IMSI: "001010000000001", NasIP: "192.168.10.1"},
		{UUID: "uuid-2", IMSI: "001010000000002", NasIP: "192.168.10.2"},
	}
	for _, s := range sessions {
		data, _ := json.Marshal(s)
		client.Set(ctx, SessionKey(s.UUID), data, 0)
	}

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

	// セッション追加
	data, _ := json.Marshal(model.Session{UUID: "uuid-1", IMSI: "001010000000001"})
	client.Set(ctx, SessionKey("uuid-1"), data, 0)

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

	// セッションとインデックスを追加
	s1 := model.Session{UUID: "uuid-1", IMSI: imsi, NasIP: "192.168.10.1"}
	data, _ := json.Marshal(s1)
	client.Set(ctx, SessionKey("uuid-1"), data, 0)
	client.SAdd(ctx, UserIndexKey(imsi), "uuid-1")

	sessions, err = ss.GetByIMSI(ctx, imsi)
	if err != nil {
		t.Fatalf("GetByIMSI() error = %v", err)
	}
	if len(sessions) != 1 {
		t.Errorf("GetByIMSI() len = %d, want 1", len(sessions))
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

func TestSessionStore_List_WithNilEntry(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ss := NewSessionStore(client)
	ctx := context.Background()

	// 正常なセッション
	s1 := model.Session{UUID: "uuid-1", IMSI: "001010000000001"}
	data, _ := json.Marshal(s1)
	client.Set(ctx, SessionKey("uuid-1"), data, 0)

	// 不正なJSONのセッション
	client.Set(ctx, SessionKey("uuid-2"), "invalid json", 0)

	list, err := ss.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	// 不正JSONはスキップされるので1件のみ
	if len(list) != 1 {
		t.Errorf("List() len = %d, want 1", len(list))
	}
}

// redisパッケージのインポートを使用していることを保証
var _ = redis.Nil
