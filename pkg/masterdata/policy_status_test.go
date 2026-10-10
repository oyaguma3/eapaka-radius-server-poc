package masterdata

import (
	"context"
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestPolicyStore_Status_DefaultActive(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	// status フィールドのない既存のポリシーは active とみなす
	mr.HSet("policy:001010000000001", "default", "deny", "rules", "[]")
	got, err := ps.Get(ctx, "001010000000001")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Status != model.PolicyStatusActive {
		t.Errorf("Get().Status = %q, want active", got.Status)
	}

	// Create は status を書き込まない
	if err := ps.Create(ctx, &model.Policy{IMSI: "001010000000002", Default: "deny", Status: model.PolicyStatusSuspended}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if mr.HGet("policy:001010000000002", "status") != "" {
		t.Error("Create() should not write status")
	}
}

func TestPolicyStore_SetStatus(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()
	imsi := "001010000000001"
	key := "policy:" + imsi

	if err := ps.Create(ctx, &model.Policy{IMSI: imsi, Default: "allow"}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// status がないときに active にしても書き込まない
	prev, err := ps.SetStatus(ctx, imsi, model.PolicyStatusActive)
	if err != nil || prev != model.PolicyStatusActive {
		t.Fatalf("SetStatus(active) = %q, %v; want active, nil", prev, err)
	}
	if mr.HGet(key, "status") != "" {
		t.Error("SetStatus(active) on absent status should not write")
	}

	// 停止
	prev, err = ps.SetStatus(ctx, imsi, model.PolicyStatusSuspended)
	if err != nil || prev != model.PolicyStatusActive {
		t.Fatalf("SetStatus(suspended) = %q, %v; want active, nil", prev, err)
	}
	if got := mr.HGet(key, "status"); got != model.PolicyStatusSuspended {
		t.Errorf("status = %q, want suspended", got)
	}

	// 同じ状態への変更
	prev, err = ps.SetStatus(ctx, imsi, model.PolicyStatusSuspended)
	if err != nil || prev != model.PolicyStatusSuspended {
		t.Fatalf("SetStatus(suspended) again = %q, %v; want suspended, nil", prev, err)
	}

	// Update・Put（全体の置き換え）では status は変わらない
	if err := ps.Update(ctx, &model.Policy{IMSI: imsi, Default: "deny", Status: model.PolicyStatusActive}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if _, err := ps.Put(ctx, &model.Policy{IMSI: imsi, Default: "deny"}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, err := ps.Get(ctx, imsi)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !got.IsSuspended() || got.Default != "deny" {
		t.Errorf("after Update/Put: Status = %q, Default = %q; want suspended, deny", got.Status, got.Default)
	}

	// 再開
	prev, err = ps.SetStatus(ctx, imsi, model.PolicyStatusActive)
	if err != nil || prev != model.PolicyStatusSuspended {
		t.Fatalf("SetStatus(active) = %q, %v; want suspended, nil", prev, err)
	}
	if got := mr.HGet(key, "status"); got != model.PolicyStatusActive {
		t.Errorf("status = %q, want active", got)
	}
}

func TestPolicyStore_SetStatus_NotFound(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	_, err := ps.SetStatus(context.Background(), "001010000000009", model.PolicyStatusSuspended)
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("SetStatus() error = %v, want ErrPolicyNotFound", err)
	}
	// 存在しないキーを status だけで作らない
	if mr.Exists("policy:001010000000009") {
		t.Error("SetStatus() should not create the key")
	}
}

func TestPolicyStore_SetStatus_Error(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()

	mr.Close()
	ps := NewPolicyStore(client)
	if _, err := ps.SetStatus(context.Background(), "001010000000001", model.PolicyStatusSuspended); err == nil || errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("SetStatus() error = %v, want connection error", err)
	}
}

func TestPolicyStore_BulkCreate_Status(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	// 既存の停止中のポリシー
	mr.HSet("policy:001010000000001", "default", "deny", "rules", "[]", "status", "suspended")

	policies := []*model.Policy{
		// status の指定なし: 既存の状態を変えない
		{IMSI: "001010000000001", Default: "allow"},
		// status の指定なし: 新規は status なし（active）
		{IMSI: "001010000000002", Default: "allow"},
		// status の指定あり
		{IMSI: "001010000000003", Default: "deny", Status: model.PolicyStatusSuspended},
	}
	if err := ps.BulkCreate(ctx, policies); err != nil {
		t.Fatalf("BulkCreate() error = %v", err)
	}

	want := map[string]string{
		"001010000000001": model.PolicyStatusSuspended,
		"001010000000002": model.PolicyStatusActive,
		"001010000000003": model.PolicyStatusSuspended,
	}
	list, err := ps.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != len(want) {
		t.Fatalf("List() len = %d, want %d", len(list), len(want))
	}
	for _, p := range list {
		if p.Status != want[p.IMSI] {
			t.Errorf("List() %s Status = %q, want %q", p.IMSI, p.Status, want[p.IMSI])
		}
	}

	page, err := ps.ListPage(ctx, "", "", 10)
	if err != nil {
		t.Fatalf("ListPage() error = %v", err)
	}
	for _, p := range page.Items {
		if p.Status != want[p.IMSI] {
			t.Errorf("ListPage() %s Status = %q, want %q", p.IMSI, p.Status, want[p.IMSI])
		}
	}
	if mr.HGet("policy:001010000000002", "status") != "" {
		t.Error("BulkCreate() without status should not write status")
	}
}
