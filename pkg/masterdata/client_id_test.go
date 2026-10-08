package masterdata

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

// TestClientStore_CreateAssignsID は、作成時にIDを 1 から順に採番し、IDからIPを引く索引を作ることを確認する。
func TestClientStore_CreateAssignsID(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()
	cs := NewClientStore(client)
	ctx := context.Background()

	for i, ip := range []string{"192.168.10.1", "192.168.10.2"} {
		c := &model.RadiusClient{IP: ip, Secret: "s", Name: "AP"}
		if err := cs.Create(ctx, c); err != nil {
			t.Fatalf("Create(%s) error = %v", ip, err)
		}
		want := int64(i + 1)
		if c.ID != want {
			t.Errorf("Create(%s) ID = %d, want %d", ip, c.ID, want)
		}
		if got, _ := mr.Get(ClientIndexKey(want)); got != ip {
			t.Errorf("index %d = %q, want %q", want, got, ip)
		}
		if got := mr.HGet(ClientKey(ip), "id"); got != strconv.FormatInt(want, 10) {
			t.Errorf("id field = %q, want %d", got, want)
		}
	}

	// 既に存在するIPは採番しない（カウンターも進めない）
	if err := cs.Create(ctx, &model.RadiusClient{IP: "192.168.10.1", Secret: "x", Name: "X"}); !errors.Is(err, ErrClientExists) {
		t.Errorf("Create() duplicate error = %v, want ErrClientExists", err)
	}
	if got, _ := mr.Get(KeyClientSeq); got != "2" {
		t.Errorf("seq = %q, want 2", got)
	}

	// client:* の SCAN（一覧・件数）に索引とカウンターが混ざらない
	if n, err := cs.Count(ctx); err != nil || n != 2 {
		t.Errorf("Count() = %d, %v, want 2", n, err)
	}
	list, err := cs.List(ctx)
	if err != nil || len(list) != 2 {
		t.Fatalf("List() = %v, %v", list, err)
	}
	for _, c := range list {
		if c.ID == 0 {
			t.Errorf("List() returned a client without ID: %+v", c)
		}
	}
}

func TestClientStore_GetByID(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()
	cs := NewClientStore(client)
	ctx := context.Background()

	c := &model.RadiusClient{IP: "10.0.0.1", Secret: "s", Name: "AP", Vendor: "v"}
	if err := cs.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := cs.GetByID(ctx, c.ID)
	if err != nil || got.IP != "10.0.0.1" || got.Name != "AP" || got.ID != c.ID {
		t.Fatalf("GetByID() = %+v, %v", got, err)
	}

	if _, err := cs.GetByID(ctx, 99); !errors.Is(err, ErrClientNotFound) {
		t.Errorf("GetByID(99) error = %v, want ErrClientNotFound", err)
	}

	// 索引が古い（指す先のクライアントのIDが違う・クライアントがない）場合は存在しないものとして扱う
	mr.Set(ClientIndexKey(50), "10.0.0.1")
	if _, err := cs.GetByID(ctx, 50); !errors.Is(err, ErrClientNotFound) {
		t.Errorf("GetByID(stale) error = %v, want ErrClientNotFound", err)
	}
	mr.Set(ClientIndexKey(51), "10.9.9.9")
	if _, err := cs.GetByID(ctx, 51); !errors.Is(err, ErrClientNotFound) {
		t.Errorf("GetByID(dangling) error = %v, want ErrClientNotFound", err)
	}

	mr.SetError("forced error")
	if _, err := cs.GetByID(ctx, c.ID); err == nil || errors.Is(err, ErrClientNotFound) {
		t.Errorf("GetByID() error = %v, want a Valkey error", err)
	}
}

// TestClientStore_PatchIP は、IPの変更でキーを付け替え、IDと他のフィールドを引き継ぐことを確認する。
func TestClientStore_PatchIP(t *testing.T) {
	str := func(s string) *string { return &s }
	mr, client := newTestRedis(t)
	defer client.Close()
	cs := NewClientStore(client)
	ctx := context.Background()

	a := &model.RadiusClient{IP: "10.0.0.1", Secret: "s1", Name: "A"}
	b := &model.RadiusClient{IP: "10.0.0.2", Secret: "s2", Name: "B"}
	for _, c := range []*model.RadiusClient{a, b} {
		if err := cs.Create(ctx, c); err != nil {
			t.Fatal(err)
		}
	}

	if err := cs.Patch(ctx, "10.0.0.1", &ClientPatch{IP: str("10.0.0.3"), Name: str("A2")}); err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if mr.Exists(ClientKey("10.0.0.1")) {
		t.Error("old key still exists")
	}
	got, err := cs.GetByID(ctx, a.ID)
	if err != nil || got.IP != "10.0.0.3" || got.Name != "A2" || got.Secret != "s1" {
		t.Fatalf("GetByID() after IP change = %+v, %v", got, err)
	}

	// 変更後のIPのクライアントが既に存在すれば、何も変えない
	if err := cs.Patch(ctx, "10.0.0.3", &ClientPatch{IP: str("10.0.0.2"), Name: str("X")}); !errors.Is(err, ErrClientExists) {
		t.Errorf("Patch() conflict error = %v, want ErrClientExists", err)
	}
	if got := mr.HGet(ClientKey("10.0.0.3"), "name"); got != "A2" {
		t.Errorf("name after conflict = %q, want A2", got)
	}
	if got, _ := mr.Get(ClientIndexKey(b.ID)); got != "10.0.0.2" {
		t.Errorf("index of b = %q", got)
	}

	// IP に同じ値を指定してもよい（フィールドだけを書き換える）
	if err := cs.Patch(ctx, "10.0.0.3", &ClientPatch{IP: str("10.0.0.3"), Vendor: str("v")}); err != nil {
		t.Errorf("Patch() same IP error = %v", err)
	}
	// IP だけの変更
	if err := cs.Patch(ctx, "10.0.0.3", &ClientPatch{IP: str("10.0.0.4")}); err != nil {
		t.Errorf("Patch() IP only error = %v", err)
	}
	if got, _ := mr.Get(ClientIndexKey(a.ID)); got != "10.0.0.4" {
		t.Errorf("index of a = %q, want 10.0.0.4", got)
	}

	if err := cs.Patch(ctx, "10.0.0.9", &ClientPatch{IP: str("10.0.0.8")}); !errors.Is(err, ErrClientNotFound) {
		t.Errorf("Patch() not found error = %v, want ErrClientNotFound", err)
	}
	if mr.Exists(ClientKey("10.0.0.8")) {
		t.Error("Patch() created a key")
	}
}

func TestClientStore_DeleteRemovesIndex(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()
	cs := NewClientStore(client)
	ctx := context.Background()

	c := &model.RadiusClient{IP: "10.0.0.1", Secret: "s", Name: "A"}
	if err := cs.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := cs.Delete(ctx, "10.0.0.1"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if mr.Exists(ClientIndexKey(c.ID)) || mr.Exists(ClientKey("10.0.0.1")) {
		t.Error("Delete() left keys")
	}

	// 索引が別のIPを指している場合は、その索引を消さない
	mr.HSet(ClientKey("10.0.0.2"), "id", "7", "secret", "s")
	mr.Set(ClientIndexKey(7), "10.0.0.5")
	if err := cs.Delete(ctx, "10.0.0.2"); err != nil {
		t.Fatal(err)
	}
	if !mr.Exists(ClientIndexKey(7)) {
		t.Error("Delete() removed an index of another client")
	}

	mr.SetError("forced error")
	if err := cs.Delete(ctx, "10.0.0.1"); err == nil {
		t.Error("Delete() expected error")
	}
}

// TestClientStore_EnsureIDs は、IDの導入前のクライアントへの採番と、何度実行しても結果が同じことを確認する。
func TestClientStore_EnsureIDs(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()
	cs := NewClientStore(client)
	ctx := context.Background()

	// IDの導入前のデータ（id なし）と、IDを持つが索引のないデータ・カウンターより大きいIDのデータ
	mr.HSet(ClientKey("10.0.0.1"), "secret", "s", "name", "A", "vendor", "")
	mr.HSet(ClientKey("10.0.0.2"), "secret", "s", "name", "B", "vendor", "")
	mr.HSet(ClientKey("10.0.0.3"), "id", "10", "secret", "s", "name", "C", "vendor", "")

	n, err := cs.EnsureIDs(ctx)
	if err != nil || n != 2 {
		t.Fatalf("EnsureIDs() = %d, %v, want 2", n, err)
	}
	if got, _ := mr.Get(ClientIndexKey(10)); got != "10.0.0.3" {
		t.Errorf("repaired index = %q", got)
	}
	list, err := cs.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int64]bool{}
	for _, c := range list {
		if c.ID == 0 || seen[c.ID] {
			t.Errorf("bad or duplicate ID: %+v", c)
		}
		seen[c.ID] = true
		if got, err := cs.GetByID(ctx, c.ID); err != nil || got.IP != c.IP {
			t.Errorf("GetByID(%d) = %+v, %v", c.ID, got, err)
		}
	}

	// 2回目は何もしない
	if n, err := cs.EnsureIDs(ctx); err != nil || n != 0 {
		t.Errorf("second EnsureIDs() = %d, %v, want 0", n, err)
	}
	// カウンターは既存の最大のID以上になり、次の作成は重複しない
	c := &model.RadiusClient{IP: "10.0.0.4", Secret: "s", Name: "D"}
	if err := cs.Create(ctx, c); err != nil {
		t.Fatal(err)
	}
	if seen[c.ID] || c.ID <= 10 {
		t.Errorf("new ID = %d collides with %v", c.ID, seen)
	}

	mr.SetError("forced error")
	if _, err := cs.EnsureIDs(ctx); err == nil {
		t.Error("EnsureIDs() expected error")
	}
}

// TestClientStore_BulkCreateKeepsID は、CSV インポートで既存のIDを引き継ぎ、新しいクライアントに採番することを確認する。
func TestClientStore_BulkCreateKeepsID(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()
	cs := NewClientStore(client)
	ctx := context.Background()

	existing := &model.RadiusClient{IP: "10.0.0.1", Secret: "old", Name: "A"}
	if err := cs.Create(ctx, existing); err != nil {
		t.Fatal(err)
	}
	clients := []*model.RadiusClient{
		{IP: "10.0.0.1", Secret: "new", Name: "A2"},
		{IP: "10.0.0.2", Secret: "s", Name: "B"},
	}
	if err := cs.BulkCreate(ctx, clients); err != nil {
		t.Fatalf("BulkCreate() error = %v", err)
	}
	if clients[0].ID != existing.ID {
		t.Errorf("existing ID = %d, want %d", clients[0].ID, existing.ID)
	}
	if clients[1].ID == 0 || clients[1].ID == existing.ID {
		t.Errorf("new ID = %d", clients[1].ID)
	}
	got, err := cs.GetByID(ctx, existing.ID)
	if err != nil || got.Secret != "new" || got.Name != "A2" {
		t.Errorf("GetByID() = %+v, %v", got, err)
	}
}
