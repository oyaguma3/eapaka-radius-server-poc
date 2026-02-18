package store

import (
	"context"
	"encoding/json"
	"testing"

	tuimodel "github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/model"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestStatisticsStore_Refresh(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ctx := context.Background()

	subStore := NewSubscriberStore(client)
	clientStore := NewClientStore(client)
	policyStore := NewPolicyStore(client)
	sessionStore := NewSessionStore(client)

	statsStore := NewStatisticsStore(subStore, clientStore, policyStore, sessionStore)

	// 空の状態
	stats, err := statsStore.Refresh(ctx)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if stats.SubscriberCount != 0 {
		t.Errorf("SubscriberCount = %d, want 0", stats.SubscriberCount)
	}

	// データ追加
	subStore.Create(ctx, &model.Subscriber{IMSI: "001010000000001", Ki: "k", OPc: "o", AMF: "a", SQN: "s"})
	clientStore.Create(ctx, &model.RadiusClient{IP: "192.168.1.1", Secret: "s", Name: "n", Vendor: "v"})
	policyStore.Upsert(ctx, &tuimodel.Policy{IMSI: "001010000000001", Default: "allow", Rules: []tuimodel.PolicyRule{}})

	sessData, _ := json.Marshal(model.Session{UUID: "uuid-1", IMSI: "001010000000001"})
	client.Set(ctx, SessionKey("uuid-1"), sessData, 0)

	stats, err = statsStore.Refresh(ctx)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if stats.SubscriberCount != 1 {
		t.Errorf("SubscriberCount = %d, want 1", stats.SubscriberCount)
	}
	if stats.ClientCount != 1 {
		t.Errorf("ClientCount = %d, want 1", stats.ClientCount)
	}
	if stats.PolicyCount != 1 {
		t.Errorf("PolicyCount = %d, want 1", stats.PolicyCount)
	}
	if stats.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1", stats.SessionCount)
	}
}

func TestStatisticsStore_Get_Cache(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ctx := context.Background()

	subStore := NewSubscriberStore(client)
	clientStore := NewClientStore(client)
	policyStore := NewPolicyStore(client)
	sessionStore := NewSessionStore(client)

	statsStore := NewStatisticsStore(subStore, clientStore, policyStore, sessionStore)

	// 初回Get（キャッシュなし → Refresh呼び出し）
	stats1, err := statsStore.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	// データ追加
	subStore.Create(ctx, &model.Subscriber{IMSI: "001010000000001", Ki: "k", OPc: "o", AMF: "a", SQN: "s"})

	// 2回目Get（キャッシュあり → キャッシュ値が返る）
	stats2, err := statsStore.Get(ctx)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	// キャッシュが効いているので、追加前のカウントが返る
	if stats2.SubscriberCount != stats1.SubscriberCount {
		t.Errorf("cached Get should return same count, got %d vs %d", stats2.SubscriberCount, stats1.SubscriberCount)
	}
}

func TestStatisticsStore_ClearCache(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ctx := context.Background()

	subStore := NewSubscriberStore(client)
	clientStore := NewClientStore(client)
	policyStore := NewPolicyStore(client)
	sessionStore := NewSessionStore(client)

	statsStore := NewStatisticsStore(subStore, clientStore, policyStore, sessionStore)

	// Refreshでキャッシュ設定
	statsStore.Refresh(ctx)

	// ClearCache
	statsStore.ClearCache()

	// データ追加後にGetするとRefreshされる
	subStore.Create(ctx, &model.Subscriber{IMSI: "001010000000001", Ki: "k", OPc: "o", AMF: "a", SQN: "s"})

	stats, _ := statsStore.Get(ctx)
	if stats.SubscriberCount != 1 {
		t.Errorf("after ClearCache, SubscriberCount = %d, want 1", stats.SubscriberCount)
	}
}
