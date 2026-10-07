package service

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
)

func TestCreateSubscriber(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()

	sub, err := svc.CreateSubscriber(ctx, testActor, dto.SubscriberCreate{IMSI: ptr(testIMSI), Ki: ptr(testKi), OPc: ptr(" " + testOPc + " ")})
	if err != nil {
		t.Fatalf("CreateSubscriber() error = %v", err)
	}
	// 16進は大文字に正規化して保存し、AMF / SQN は既定値になる
	key := masterdata.SubscriberKey(testIMSI)
	want := map[string]string{
		"ki": strings.ToUpper(testKi), "opc": strings.ToUpper(testOPc),
		"amf": "8000", "sqn": "000000000000", "created_at": "2026-10-07T03:00:00Z",
	}
	for f, v := range want {
		if got := mr.HGet(key, f); got != v {
			t.Errorf("%s = %q, want %q", f, got, v)
		}
	}
	if sub.IMSI != testIMSI || sub.CreatedAt != "2026-10-07T03:00:00Z" {
		t.Errorf("CreateSubscriber() = %+v", sub)
	}

	e := lastAudit(t, buf)
	wantAudit := map[string]any{
		"msg": "subscriber created", "event_id": "AUDIT_LOG", "app": "provisioning-api", "operation": "create",
		"target_type": "subscriber", "target_key": key, "target_imsi": testIMSI,
		"admin_user": "alice", "mgmt_client": "bff-01", "trace_id": "trace-1", "details": "amf=8000, sqn=000000000000",
	}
	for k, v := range wantAudit {
		if e[k] != v {
			t.Errorf("audit %s = %v, want %v", k, e[k], v)
		}
	}
	// 秘密の値は監査ログに出さない
	if out := strings.ToUpper(buf.String()); strings.Contains(out, strings.ToUpper(testKi)) || strings.Contains(out, strings.ToUpper(testOPc)) {
		t.Errorf("audit log contains a key: %s", buf.String())
	}

	// 同じ IMSI は既に存在する
	_, err = svc.CreateSubscriber(ctx, testActor, dto.SubscriberCreate{IMSI: ptr(testIMSI), Ki: ptr(testKi), OPc: ptr(testOPc)})
	if !errors.Is(err, masterdata.ErrSubscriberExists) {
		t.Errorf("CreateSubscriber() duplicate error = %v, want ErrSubscriberExists", err)
	}
	if n := len(auditEntries(t, buf)); n != 1 {
		t.Errorf("audit entries = %d, want 1 (failures are not recorded)", n)
	}
}

func TestCreateSubscriber_AMFAndSQN(t *testing.T) {
	svc, mr, _ := newTestService(t)
	_, err := svc.CreateSubscriber(context.Background(), testActor, dto.SubscriberCreate{
		IMSI: ptr(testIMSI), Ki: ptr(testKi), OPc: ptr(testOPc), AMF: ptr("b9b9"), SQN: ptr("ff9bb4d0b607"),
	})
	if err != nil {
		t.Fatalf("CreateSubscriber() error = %v", err)
	}
	key := masterdata.SubscriberKey(testIMSI)
	if mr.HGet(key, "amf") != "B9B9" || mr.HGet(key, "sqn") != "FF9BB4D0B607" {
		t.Errorf("amf = %s, sqn = %s", mr.HGet(key, "amf"), mr.HGet(key, "sqn"))
	}
}

func TestCreateSubscriber_Validation(t *testing.T) {
	tests := []struct {
		name       string
		req        dto.SubscriberCreate
		wantCause  string
		wantParams []string
	}{
		{"all missing", dto.SubscriberCreate{}, dto.CauseMandatoryIEMissing, []string{"imsi", "ki", "opc"}},
		{"missing wins over incorrect", dto.SubscriberCreate{IMSI: ptr("123"), Ki: ptr(testKi)}, dto.CauseMandatoryIEMissing, []string{"opc", "imsi"}},
		{"mandatory incorrect", dto.SubscriberCreate{IMSI: ptr(testIMSI), Ki: ptr("xyz"), OPc: ptr("")}, dto.CauseMandatoryIEIncorrect, []string{"ki", "opc"}},
		{"optional incorrect", dto.SubscriberCreate{IMSI: ptr(testIMSI), Ki: ptr(testKi), OPc: ptr(testOPc), AMF: ptr("80000"), SQN: ptr("zz")}, dto.CauseOptionalIEIncorrect, []string{"amf", "sqn"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, mr, buf := newTestService(t)
			_, err := svc.CreateSubscriber(context.Background(), testActor, tt.req)
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

// seedSubscriber は加入者を Valkey に直接書き込む。
func seedSubscriber(t *testing.T, svc *Service) {
	t.Helper()
	_, err := svc.CreateSubscriber(context.Background(), testActor, dto.SubscriberCreate{
		IMSI: ptr(testIMSI), Ki: ptr(testKi), OPc: ptr(testOPc), SQN: ptr("ff9bb4d0b607"),
	})
	if err != nil {
		t.Fatalf("CreateSubscriber() error = %v", err)
	}
}

func TestGetSubscriberAndKeys(t *testing.T) {
	svc, _, buf := newTestService(t)
	ctx := context.Background()
	seedSubscriber(t, svc)
	buf.Reset()

	sub, err := svc.GetSubscriber(ctx, testIMSI)
	if err != nil || sub.SQN != "FF9BB4D0B607" {
		t.Fatalf("GetSubscriber() = %+v, %v", sub, err)
	}
	if buf.Len() != 0 {
		t.Errorf("GetSubscriber() wrote an audit log: %s", buf.String())
	}

	sub, err = svc.GetSubscriberKeys(ctx, testActor, testIMSI)
	if err != nil || sub.Ki != strings.ToUpper(testKi) {
		t.Fatalf("GetSubscriberKeys() = %+v, %v", sub, err)
	}
	e := lastAudit(t, buf)
	if e["operation"] != "read" || e["details"] != "ki,opc" || e["msg"] != "subscriber secret read" || e["target_imsi"] != testIMSI {
		t.Errorf("audit = %v", e)
	}

	if _, err := svc.GetSubscriberKeys(ctx, testActor, "001010000000099"); !errors.Is(err, masterdata.ErrSubscriberNotFound) {
		t.Errorf("GetSubscriberKeys() error = %v, want ErrSubscriberNotFound", err)
	}
	if _, err := svc.GetSubscriber(ctx, "00101"); validationError(t, err).Cause() != dto.CauseMandatoryIEIncorrect {
		t.Errorf("GetSubscriber() bad imsi error = %v", err)
	}
}

func TestUpdateSubscriber(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	seedSubscriber(t, svc)
	key := masterdata.SubscriberKey(testIMSI)
	// 認証で SQN が進んだ状態
	mr.HSet(key, "sqn", "FF9BB4D0B627")

	sub, err := svc.UpdateSubscriber(ctx, testActor, testIMSI, dto.SubscriberUpdate{AMF: set("b9b9"), Ki: set(testKi), OPc: set("00112233445566778899aabbccddeeff")})
	if err != nil {
		t.Fatalf("UpdateSubscriber() error = %v", err)
	}
	// sqn を指定しなければ SQN には触れない
	if sub.SQN != "FF9BB4D0B627" || mr.HGet(key, "sqn") != "FF9BB4D0B627" {
		t.Errorf("sqn = %s, want FF9BB4D0B627", mr.HGet(key, "sqn"))
	}
	if mr.HGet(key, "amf") != "B9B9" || mr.HGet(key, "opc") != "00112233445566778899AABBCCDDEEFF" {
		t.Errorf("amf = %s, opc = %s", mr.HGet(key, "amf"), mr.HGet(key, "opc"))
	}
	e := lastAudit(t, buf)
	if e["operation"] != "update" || e["details"] != "ki: unchanged, opc: changed, amf: 8000 -> b9b9" {
		t.Errorf("audit = %v", e)
	}

	// sqn を指定した場合はそのまま書き換える
	if _, err := svc.UpdateSubscriber(ctx, testActor, testIMSI, dto.SubscriberUpdate{SQN: set("000000000020")}); err != nil {
		t.Fatalf("UpdateSubscriber() error = %v", err)
	}
	if got := mr.HGet(key, "sqn"); got != "000000000020" {
		t.Errorf("sqn = %s", got)
	}
	if e := lastAudit(t, buf); e["details"] != "sqn: ff9bb4d0b627 -> 000000000020" {
		t.Errorf("audit details = %v", e["details"])
	}
}

func TestUpdateSubscriber_Errors(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	seedSubscriber(t, svc)
	buf.Reset()

	tests := []struct {
		name       string
		imsi       string
		upd        dto.SubscriberUpdate
		wantCause  string
		wantParams []string
	}{
		{"bad imsi", "abc", dto.SubscriberUpdate{AMF: set("8000")}, dto.CauseMandatoryIEIncorrect, []string{"imsi"}},
		{"empty patch", testIMSI, dto.SubscriberUpdate{}, dto.CauseMandatoryIEMissing, nil},
		{"null and bad values", testIMSI, dto.SubscriberUpdate{Ki: dto.Optional[string]{Set: true, Null: true}, SQN: set("1")}, dto.CauseOptionalIEIncorrect, []string{"ki", "sqn"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateSubscriber(ctx, testActor, tt.imsi, tt.upd)
			ve := validationError(t, err)
			if ve.Cause() != tt.wantCause || !slices.Equal(paramNames(ve), tt.wantParams) {
				t.Errorf("cause = %s, params = %v", ve.Cause(), paramNames(ve))
			}
		})
	}

	if _, err := svc.UpdateSubscriber(ctx, testActor, "001010000000099", dto.SubscriberUpdate{AMF: set("8000")}); !errors.Is(err, masterdata.ErrSubscriberNotFound) {
		t.Errorf("UpdateSubscriber() error = %v, want ErrSubscriberNotFound", err)
	}
	if mr.Exists(masterdata.SubscriberKey("001010000000099")) {
		t.Error("UpdateSubscriber() created a subscriber")
	}

	mr.SetError("forced error")
	if _, err := svc.UpdateSubscriber(ctx, testActor, testIMSI, dto.SubscriberUpdate{AMF: set("8000")}); err == nil {
		t.Error("UpdateSubscriber() expected error")
	}
	if buf.Len() != 0 {
		t.Errorf("failed updates wrote audit logs: %s", buf.String())
	}
}

func TestDeleteSubscriber(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	seedSubscriber(t, svc)
	mr.HSet(masterdata.PolicyKey(testIMSI), "default", "deny")

	if err := svc.DeleteSubscriber(ctx, testActor, testIMSI); err != nil {
		t.Fatalf("DeleteSubscriber() error = %v", err)
	}
	if mr.Exists(masterdata.SubscriberKey(testIMSI)) {
		t.Error("subscriber still exists")
	}
	// 認可ポリシーは削除しない
	if !mr.Exists(masterdata.PolicyKey(testIMSI)) {
		t.Error("policy was deleted")
	}
	if e := lastAudit(t, buf); e["operation"] != "delete" || e["details"] != "amf=8000, sqn=ff9bb4d0b607" {
		t.Errorf("audit = %v", e)
	}

	if err := svc.DeleteSubscriber(ctx, testActor, testIMSI); !errors.Is(err, masterdata.ErrSubscriberNotFound) {
		t.Errorf("DeleteSubscriber() error = %v, want ErrSubscriberNotFound", err)
	}
	if err := svc.DeleteSubscriber(ctx, testActor, "1"); validationError(t, err).Cause() != dto.CauseMandatoryIEIncorrect {
		t.Errorf("DeleteSubscriber() bad imsi error = %v", err)
	}
}

func TestListSubscribers(t *testing.T) {
	svc, _, _ := newTestService(t)
	ctx := context.Background()
	seedSubscriber(t, svc)

	page, err := svc.ListSubscribers(ctx, dto.ListQuery{Prefix: "00101"})
	if err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("ListSubscribers() = %+v, %v", page, err)
	}
	if _, err := svc.ListSubscribers(ctx, dto.ListQuery{Limit: "x"}); validationError(t, err).Cause() != dto.CauseInvalidQueryParam {
		t.Errorf("ListSubscribers() error = %v", err)
	}
}
