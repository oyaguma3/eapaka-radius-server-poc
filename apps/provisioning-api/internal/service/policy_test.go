package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
)

// testPolicy は2件のルールを持つ認可ポリシーの要求を返す。
func testPolicy() dto.PolicyPut {
	return dto.PolicyPut{
		Default: ptr("deny"),
		Rules: []dto.PolicyRuleInput{
			{NasID: ptr("AP-OFFICE-01"), AllowedSSIDs: []string{"CORP-WIFI"}, VlanID: ptr("100"), SessionTimeout: ptr(3600)},
			{NasID: ptr("*"), AllowedSSIDs: []string{" GUEST-WIFI "}},
		},
	}
}

func TestPutPolicy(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	key := masterdata.PolicyKey(testIMSI)

	// 作成（加入者がなくてもよい）
	p, created, err := svc.PutPolicy(ctx, testActor, testIMSI, testPolicy())
	if err != nil || !created {
		t.Fatalf("PutPolicy() = %v, %v, want created", created, err)
	}
	if p.Rules[1].AllowedSSIDs[0] != "GUEST-WIFI" {
		t.Errorf("SSID was not trimmed: %q", p.Rules[1].AllowedSSIDs[0])
	}
	// Valkey へは D-02 の形式（snake_case の JSON 文字列）で保存する
	wantRules := `[{"nas_id":"AP-OFFICE-01","allowed_ssids":["CORP-WIFI"],"vlan_id":"100","session_timeout":3600},{"nas_id":"*","allowed_ssids":["GUEST-WIFI"]}]`
	if mr.HGet(key, "default") != "deny" || mr.HGet(key, "rules") != wantRules {
		t.Errorf("stored default = %s, rules = %s", mr.HGet(key, "default"), mr.HGet(key, "rules"))
	}
	if e := lastAudit(t, buf); e["operation"] != "create" || e["details"] != "default=deny, rules=2" || e["target_imsi"] != testIMSI {
		t.Errorf("audit = %v", e)
	}

	// 同じ内容で置き換え
	if _, created, err = svc.PutPolicy(ctx, testActor, testIMSI, testPolicy()); err != nil || created {
		t.Fatalf("PutPolicy() = %v, %v, want replaced", created, err)
	}
	if e := lastAudit(t, buf); e["operation"] != "update" || e["details"] != "default: deny -> deny, rules: unchanged (2 -> 2)" {
		t.Errorf("audit = %v", e)
	}

	// default を大文字で指定しても小文字に正規化する。ルールは空にできる
	if _, created, err = svc.PutPolicy(ctx, testActor, testIMSI, dto.PolicyPut{Default: ptr("ALLOW"), Rules: []dto.PolicyRuleInput{}}); err != nil || created {
		t.Fatalf("PutPolicy() = %v, %v", created, err)
	}
	if mr.HGet(key, "default") != "allow" || mr.HGet(key, "rules") != "[]" {
		t.Errorf("stored default = %s, rules = %s", mr.HGet(key, "default"), mr.HGet(key, "rules"))
	}
	if e := lastAudit(t, buf); e["details"] != "default: deny -> allow, rules: changed (2 -> 0)" {
		t.Errorf("audit details = %v", e["details"])
	}

	// 壊れた rules も置き換えられる（変更前の値は読めないので、作成と同じ形で記録する）
	mr.HSet(key, "rules", "{broken")
	if _, created, err = svc.PutPolicy(ctx, testActor, testIMSI, testPolicy()); err != nil || created {
		t.Fatalf("PutPolicy() over broken rules = %v, %v", created, err)
	}
	if e := lastAudit(t, buf); e["operation"] != "update" || e["details"] != "default=deny, rules=2" {
		t.Errorf("audit = %v", e)
	}

	mr.SetError("forced error")
	if _, _, err := svc.PutPolicy(ctx, testActor, testIMSI, testPolicy()); err == nil {
		t.Error("PutPolicy() expected error")
	}
}

func TestPutPolicy_Validation(t *testing.T) {
	tests := []struct {
		name       string
		imsi       string
		req        dto.PolicyPut
		wantCause  string
		wantParams []string
	}{
		{"bad imsi", "x", testPolicy(), dto.CauseMandatoryIEIncorrect, []string{"imsi"}},
		{"all missing", testIMSI, dto.PolicyPut{}, dto.CauseMandatoryIEMissing, []string{"default", "rules"}},
		{"rule fields missing", testIMSI, dto.PolicyPut{Default: ptr("deny"), Rules: []dto.PolicyRuleInput{{}}}, dto.CauseMandatoryIEMissing,
			[]string{"rules[0].nasId", "rules[0].allowedSsids"}},
		{"mandatory incorrect", testIMSI, dto.PolicyPut{Default: ptr("maybe"), Rules: []dto.PolicyRuleInput{
			{NasID: ptr("ap 01"), AllowedSSIDs: []string{}},
			{NasID: ptr("*"), AllowedSSIDs: []string{"OK", "", "123456789012345678901234567890123"}},
		}}, dto.CauseMandatoryIEIncorrect, []string{"default", "rules[0].nasId", "rules[0].allowedSsids", "rules[1].allowedSsids[1]", "rules[1].allowedSsids[2]"}},
		{"optional incorrect", testIMSI, dto.PolicyPut{Default: ptr("deny"), Rules: []dto.PolicyRuleInput{
			{NasID: ptr("*"), AllowedSSIDs: []string{"A"}, VlanID: ptr("+5"), SessionTimeout: ptr(-1)},
			{NasID: ptr("*"), AllowedSSIDs: []string{"A"}, VlanID: ptr("4095"), SessionTimeout: ptr(86401)},
		}}, dto.CauseOptionalIEIncorrect, []string{"rules[0].vlanId", "rules[0].sessionTimeout", "rules[1].vlanId", "rules[1].sessionTimeout"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, mr, buf := newTestService(t)
			_, _, err := svc.PutPolicy(context.Background(), testActor, tt.imsi, tt.req)
			ve := validationError(t, err)
			if ve.Cause() != tt.wantCause || !slices.Equal(paramNames(ve), tt.wantParams) {
				t.Errorf("cause = %s, params = %v, want %s, %v", ve.Cause(), paramNames(ve), tt.wantCause, tt.wantParams)
			}
			if len(mr.Keys()) != 0 || buf.Len() != 0 {
				t.Errorf("keys = %v, audit = %q", mr.Keys(), buf.String())
			}
		})
	}
}

func TestGetDeleteListPolicies(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	if _, _, err := svc.PutPolicy(ctx, testActor, testIMSI, testPolicy()); err != nil {
		t.Fatalf("PutPolicy() error = %v", err)
	}

	p, err := svc.GetPolicy(ctx, testIMSI)
	if err != nil || p.Default != "deny" || len(p.Rules) != 2 {
		t.Fatalf("GetPolicy() = %+v, %v", p, err)
	}
	if _, err := svc.GetPolicy(ctx, "1"); validationError(t, err).Cause() != dto.CauseMandatoryIEIncorrect {
		t.Errorf("GetPolicy() bad imsi error = %v", err)
	}

	page, err := svc.ListPolicies(ctx, dto.ListQuery{Limit: "10"})
	if err != nil || page.Total != 1 || page.Items[0].IMSI != testIMSI {
		t.Fatalf("ListPolicies() = %+v, %v", page, err)
	}
	if _, err := svc.ListPolicies(ctx, dto.ListQuery{Prefix: "x"}); validationError(t, err).Cause() != dto.CauseInvalidQueryParam {
		t.Errorf("ListPolicies() error = %v", err)
	}

	if err := svc.DeletePolicy(ctx, testActor, testIMSI); err != nil {
		t.Fatalf("DeletePolicy() error = %v", err)
	}
	if mr.Exists(masterdata.PolicyKey(testIMSI)) {
		t.Error("policy still exists")
	}
	if e := lastAudit(t, buf); e["operation"] != "delete" || e["details"] != "default=deny, rules=2" {
		t.Errorf("audit = %v", e)
	}
	if err := svc.DeletePolicy(ctx, testActor, testIMSI); !errors.Is(err, masterdata.ErrPolicyNotFound) {
		t.Errorf("DeletePolicy() error = %v, want ErrPolicyNotFound", err)
	}
	if _, err := svc.GetPolicy(ctx, testIMSI); !errors.Is(err, masterdata.ErrPolicyNotFound) {
		t.Errorf("GetPolicy() error = %v, want ErrPolicyNotFound", err)
	}
	if err := svc.DeletePolicy(ctx, testActor, "1"); validationError(t, err).Cause() != dto.CauseMandatoryIEIncorrect {
		t.Errorf("DeletePolicy() bad imsi error = %v", err)
	}
}

func TestSetPolicyStatus(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	key := masterdata.PolicyKey(testIMSI)

	// 認可ポリシーがなければ ErrPolicyNotFound
	if _, err := svc.SetPolicyStatus(ctx, testActor, testIMSI, dto.PolicyStatusPut{Status: ptr("suspended")}); !errors.Is(err, masterdata.ErrPolicyNotFound) {
		t.Fatalf("SetPolicyStatus() error = %v, want ErrPolicyNotFound", err)
	}
	if _, _, err := svc.PutPolicy(ctx, testActor, testIMSI, testPolicy()); err != nil {
		t.Fatalf("PutPolicy() error = %v", err)
	}
	n := len(auditEntries(t, buf))

	// 停止（大文字・前後の空白は正規化する）
	p, err := svc.SetPolicyStatus(ctx, testActor, testIMSI, dto.PolicyStatusPut{Status: ptr(" SUSPENDED ")})
	if err != nil || p.Status != "suspended" || len(p.Rules) != 2 {
		t.Fatalf("SetPolicyStatus() = %+v, %v", p, err)
	}
	if mr.HGet(key, "status") != "suspended" {
		t.Errorf("stored status = %q", mr.HGet(key, "status"))
	}
	if e := lastAudit(t, buf); e["operation"] != "suspend" || e["msg"] != "policy suspended" || e["details"] != "status: active -> suspended" || e["target_imsi"] != testIMSI {
		t.Errorf("audit = %v", e)
	}

	// 同じ状態への変更は監査ログに残さない
	if p, err = svc.SetPolicyStatus(ctx, testActor, testIMSI, dto.PolicyStatusPut{Status: ptr("suspended")}); err != nil || p.Status != "suspended" {
		t.Fatalf("SetPolicyStatus() again = %+v, %v", p, err)
	}
	if got := len(auditEntries(t, buf)); got != n+1 {
		t.Errorf("audit entries = %d, want %d", got, n+1)
	}

	// 置き換えでは状態は変わらず、応答に今の状態が入る
	p, _, err = svc.PutPolicy(ctx, testActor, testIMSI, testPolicy())
	if err != nil || p.Status != "suspended" {
		t.Fatalf("PutPolicy() = %+v, %v", p, err)
	}

	// 再開
	if p, err = svc.SetPolicyStatus(ctx, testActor, testIMSI, dto.PolicyStatusPut{Status: ptr("active")}); err != nil || p.Status != "active" {
		t.Fatalf("SetPolicyStatus(active) = %+v, %v", p, err)
	}
	if e := lastAudit(t, buf); e["operation"] != "resume" || e["msg"] != "policy resumed" || e["details"] != "status: suspended -> active" {
		t.Errorf("audit = %v", e)
	}

	mr.SetError("forced error")
	if _, err := svc.SetPolicyStatus(ctx, testActor, testIMSI, dto.PolicyStatusPut{Status: ptr("active")}); err == nil {
		t.Error("SetPolicyStatus() expected error")
	}
}

func TestSetPolicyStatus_Validation(t *testing.T) {
	tests := []struct {
		name      string
		imsi      string
		req       dto.PolicyStatusPut
		wantCause string
	}{
		{"bad imsi", "x", dto.PolicyStatusPut{Status: ptr("active")}, dto.CauseMandatoryIEIncorrect},
		{"missing", testIMSI, dto.PolicyStatusPut{}, dto.CauseMandatoryIEMissing},
		{"empty", testIMSI, dto.PolicyStatusPut{Status: ptr("")}, dto.CauseMandatoryIEIncorrect},
		{"unknown", testIMSI, dto.PolicyStatusPut{Status: ptr("stopped")}, dto.CauseMandatoryIEIncorrect},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, mr, buf := newTestService(t)
			mr.HSet(masterdata.PolicyKey(testIMSI), "default", "deny", "rules", "[]")
			_, err := svc.SetPolicyStatus(context.Background(), testActor, tt.imsi, tt.req)
			if ve := validationError(t, err); ve.Cause() != tt.wantCause {
				t.Errorf("cause = %s, want %s", ve.Cause(), tt.wantCause)
			}
			if mr.HGet(masterdata.PolicyKey(testIMSI), "status") != "" || buf.Len() != 0 {
				t.Errorf("status = %q, audit = %q", mr.HGet(masterdata.PolicyKey(testIMSI), "status"), buf.String())
			}
		})
	}
}
