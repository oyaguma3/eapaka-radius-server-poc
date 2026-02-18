package store

import (
	"context"
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/model"
)

func TestPolicyStore_CRUD(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	policy := &model.Policy{
		IMSI:    "001010000000001",
		Default: "allow",
		Rules:   []model.PolicyRule{},
	}

	// Create
	if err := ps.Create(ctx, policy); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Create duplicate
	if err := ps.Create(ctx, policy); err == nil {
		t.Error("Create() expected error for duplicate")
	}

	// Exists
	exists, err := ps.Exists(ctx, policy.IMSI)
	if err != nil {
		t.Fatalf("Exists() error = %v", err)
	}
	if !exists {
		t.Error("Exists() = false, want true")
	}

	// Get
	got, err := ps.Get(ctx, policy.IMSI)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Default != "allow" {
		t.Errorf("Get().Default = %s, want allow", got.Default)
	}

	// Get not found
	_, err = ps.Get(ctx, "999999999999999")
	if !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("Get() expected ErrPolicyNotFound, got: %v", err)
	}

	// Update
	policy.Default = "deny"
	policy.Rules = []model.PolicyRule{
		{NasID: "Customer01", AllowedSSIDs: []string{"TESTSSID-01"}, VlanID: "100"},
	}
	if err := ps.Update(ctx, policy); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, _ = ps.Get(ctx, policy.IMSI)
	if got.Default != "deny" {
		t.Errorf("after Update, Default = %s, want deny", got.Default)
	}
	if len(got.Rules) != 1 {
		t.Errorf("after Update, Rules len = %d, want 1", len(got.Rules))
	}

	// Update not found
	if err := ps.Update(ctx, &model.Policy{IMSI: "999999999999999"}); !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("Update() expected ErrPolicyNotFound, got: %v", err)
	}

	// Delete
	if err := ps.Delete(ctx, policy.IMSI); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Delete not found
	if err := ps.Delete(ctx, policy.IMSI); !errors.Is(err, ErrPolicyNotFound) {
		t.Errorf("Delete() expected ErrPolicyNotFound, got: %v", err)
	}
}

func TestPolicyStore_Upsert(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	policy := &model.Policy{
		IMSI:    "001010000000001",
		Default: "allow",
		Rules:   []model.PolicyRule{},
	}

	// Upsert（新規作成）
	if err := ps.Upsert(ctx, policy); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	got, _ := ps.Get(ctx, policy.IMSI)
	if got.Default != "allow" {
		t.Errorf("after Upsert, Default = %s, want allow", got.Default)
	}

	// Upsert（更新）
	policy.Default = "deny"
	if err := ps.Upsert(ctx, policy); err != nil {
		t.Fatalf("Upsert() error = %v", err)
	}

	got, _ = ps.Get(ctx, policy.IMSI)
	if got.Default != "deny" {
		t.Errorf("after Upsert update, Default = %s, want deny", got.Default)
	}
}

func TestPolicyStore_List(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	// 空リスト
	list, err := ps.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List() len = %d, want 0", len(list))
	}

	// データ追加後
	ps.Upsert(ctx, &model.Policy{IMSI: "001010000000001", Default: "allow", Rules: []model.PolicyRule{}})
	ps.Upsert(ctx, &model.Policy{IMSI: "001010000000002", Default: "deny", Rules: []model.PolicyRule{
		{NasID: "NAS1", AllowedSSIDs: []string{"SSID1"}},
	}})

	list, err = ps.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List() len = %d, want 2", len(list))
	}
}

func TestPolicyStore_BulkCreate(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	// 空リスト
	if err := ps.BulkCreate(ctx, []*model.Policy{}); err != nil {
		t.Fatalf("BulkCreate(empty) error = %v", err)
	}

	policies := []*model.Policy{
		{IMSI: "001010000000001", Default: "allow", Rules: []model.PolicyRule{}},
		{IMSI: "001010000000002", Default: "deny", Rules: []model.PolicyRule{
			{NasID: "NAS1", AllowedSSIDs: []string{"SSID1"}},
		}},
	}

	if err := ps.BulkCreate(ctx, policies); err != nil {
		t.Fatalf("BulkCreate() error = %v", err)
	}

	exists, _ := ps.Exists(ctx, "001010000000001")
	if !exists {
		t.Error("BulkCreate: policy 001010000000001 should exist")
	}
}

func TestPolicyStore_GetIMSIsWithPolicy(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	ps.Upsert(ctx, &model.Policy{IMSI: "001010000000001", Default: "allow", Rules: []model.PolicyRule{}})
	ps.Upsert(ctx, &model.Policy{IMSI: "001010000000002", Default: "deny", Rules: []model.PolicyRule{}})

	result, err := ps.GetIMSIsWithPolicy(ctx)
	if err != nil {
		t.Fatalf("GetIMSIsWithPolicy() error = %v", err)
	}
	if len(result) != 2 {
		t.Errorf("GetIMSIsWithPolicy() len = %d, want 2", len(result))
	}
	if !result["001010000000001"] {
		t.Error("expected 001010000000001 in result")
	}
}

func TestPolicyStore_Get_DefaultDeny(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	// defaultフィールドなしのポリシーを直接Valkeyに書き込み
	key := PolicyKey("001010000000099")
	client.HSet(ctx, key, map[string]any{
		"rules": "[]",
	})

	got, err := ps.Get(ctx, "001010000000099")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	// defaultフィールドがない場合は"deny"がデフォルト
	if got.Default != "deny" {
		t.Errorf("Get().Default = %s, want deny (default)", got.Default)
	}
}

func TestPolicyStore_Get_EmptyRules(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	// rulesフィールドなしのポリシー
	key := PolicyKey("001010000000098")
	client.HSet(ctx, key, map[string]any{
		"default": "allow",
	})

	got, err := ps.Get(ctx, "001010000000098")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.RulesJSON != "[]" {
		t.Errorf("Get().RulesJSON = %s, want []", got.RulesJSON)
	}
	if len(got.Rules) != 0 {
		t.Errorf("Get().Rules len = %d, want 0", len(got.Rules))
	}
}

func TestPolicyStore_BulkCreate_WithRulesJSON(t *testing.T) {
	_, client := newTestRedis(t)
	defer client.Close()

	ps := NewPolicyStore(client)
	ctx := context.Background()

	// RulesJSONのみ設定されたポリシー
	policies := []*model.Policy{
		{IMSI: "001010000000001", Default: "deny", RulesJSON: `[{"nas_id":"NAS1","allowed_ssids":["SSID1"]}]`},
	}

	if err := ps.BulkCreate(ctx, policies); err != nil {
		t.Fatalf("BulkCreate() error = %v", err)
	}

	got, _ := ps.Get(ctx, "001010000000001")
	if len(got.Rules) != 1 {
		t.Errorf("Get().Rules len = %d, want 1", len(got.Rules))
	}
}
