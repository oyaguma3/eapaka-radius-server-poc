package service

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
)

const testSecret = "c2VjcmV0LWV4YW1wbGU"

// seedClient は RADIUSクライアントを登録し、ID（文字列）を返す。
func seedClient(t *testing.T, svc *Service, ip string) string {
	t.Helper()
	c, err := svc.CreateClient(context.Background(), testActor, dto.ClientCreate{IP: ptr(ip), Secret: ptr(testSecret), Name: ptr("AP-01")})
	if err != nil {
		t.Fatalf("CreateClient(%s) error = %v", ip, err)
	}
	return strconv.FormatInt(c.ID, 10)
}

func TestValidateIP(t *testing.T) {
	for ip, ok := range map[string]bool{
		"192.168.10.1": true, "0.0.0.0": true,
		"192.168.010.1": false, "256.1.1.1": false, "": false, "::1": false, "192.168.10.1 ": false,
	} {
		if err := validateIP(ip); (err == nil) != ok {
			t.Errorf("validateIP(%q) = %v, want ok=%v", ip, err, ok)
		}
	}
}

func TestParseClientID(t *testing.T) {
	for s, ok := range map[string]bool{
		"1": true, "42": true, "999999999999999999": true,
		"0": false, "01": false, "-1": false, "a": false, "": false, "1.5": false, "9999999999999999999": false,
	} {
		_, err := parseClientID(s)
		if (err == nil) != ok {
			t.Errorf("parseClientID(%q) = %v, want ok=%v", s, err, ok)
		}
		if err != nil && validationError(t, err).Cause() != dto.CauseMandatoryIEIncorrect {
			t.Errorf("parseClientID(%q) cause = %s", s, validationError(t, err).Cause())
		}
	}
}

func TestCreateClient(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()

	c, err := svc.CreateClient(ctx, testActor, dto.ClientCreate{IP: ptr(testIP), Secret: ptr(testSecret), Name: ptr("AP-01"), Vendor: ptr("generic")})
	if err != nil {
		t.Fatalf("CreateClient() error = %v", err)
	}
	if c.ID != 1 {
		t.Errorf("ID = %d, want 1", c.ID)
	}
	key := masterdata.ClientKey(testIP)
	if mr.HGet(key, "secret") != testSecret || mr.HGet(key, "name") != "AP-01" || mr.HGet(key, "vendor") != "generic" || mr.HGet(key, "id") != "1" {
		t.Errorf("stored = %v", mr.Keys())
	}
	e := lastAudit(t, buf)
	if e["operation"] != "create" || e["target_type"] != "client" || e["target_key"] != key || e["target_id"] != "1" ||
		e["details"] != `ip=192.168.10.1, name="AP-01", vendor="generic"` {
		t.Errorf("audit = %v", e)
	}
	if _, ok := e["target_imsi"]; ok {
		t.Error("client audit has target_imsi")
	}
	if strings.Contains(buf.String(), testSecret) {
		t.Errorf("audit log contains the secret: %s", buf.String())
	}

	// vendor は省略できる（空文字）。ID は続きから採番する
	c2, err := svc.CreateClient(ctx, testActor, dto.ClientCreate{IP: ptr("10.0.0.1"), Secret: ptr("s"), Name: ptr("AP-02")})
	if err != nil {
		t.Fatalf("CreateClient() without vendor error = %v", err)
	}
	if c2.ID != 2 || mr.HGet(masterdata.ClientKey("10.0.0.1"), "vendor") != "" {
		t.Errorf("second client = %+v", c2)
	}

	if _, err := svc.CreateClient(ctx, testActor, dto.ClientCreate{IP: ptr(testIP), Secret: ptr("s"), Name: ptr("AP-03")}); !errors.Is(err, masterdata.ErrClientExists) {
		t.Errorf("CreateClient() duplicate error = %v, want ErrClientExists", err)
	}
}

func TestCreateClient_Validation(t *testing.T) {
	tests := []struct {
		name       string
		req        dto.ClientCreate
		wantCause  string
		wantParams []string
	}{
		{"all missing", dto.ClientCreate{}, dto.CauseMandatoryIEMissing, []string{"ip", "secret", "name"}},
		{"mandatory incorrect", dto.ClientCreate{IP: ptr("192.168.010.1"), Secret: ptr("has space"), Name: ptr("AP 01")}, dto.CauseMandatoryIEIncorrect, []string{"ip", "secret", "name"}},
		{"vendor incorrect", dto.ClientCreate{IP: ptr(testIP), Secret: ptr("s"), Name: ptr("AP"), Vendor: ptr("bad@vendor")}, dto.CauseOptionalIEIncorrect, []string{"vendor"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, mr, _ := newTestService(t)
			_, err := svc.CreateClient(context.Background(), testActor, tt.req)
			ve := validationError(t, err)
			if ve.Cause() != tt.wantCause || !slices.Equal(paramNames(ve), tt.wantParams) {
				t.Errorf("cause = %s, params = %v, want %s, %v", ve.Cause(), paramNames(ve), tt.wantCause, tt.wantParams)
			}
			if len(mr.Keys()) != 0 {
				t.Errorf("keys = %v", mr.Keys())
			}
		})
	}
}

func TestGetClientAndSecret(t *testing.T) {
	svc, _, buf := newTestService(t)
	ctx := context.Background()
	id := seedClient(t, svc, testIP)
	buf.Reset()

	c, err := svc.GetClient(ctx, id)
	if err != nil || c.Name != "AP-01" || c.IP != testIP {
		t.Fatalf("GetClient() = %+v, %v", c, err)
	}
	if buf.Len() != 0 {
		t.Errorf("GetClient() wrote an audit log")
	}
	c, err = svc.GetClientSecret(ctx, testActor, id)
	if err != nil || c.Secret != testSecret {
		t.Fatalf("GetClientSecret() = %+v, %v", c, err)
	}
	if e := lastAudit(t, buf); e["operation"] != "read" || e["details"] != "secret" || e["msg"] != "client secret read" || e["target_id"] != id {
		t.Errorf("audit = %v", e)
	}
	if _, err := svc.GetClientSecret(ctx, testActor, "99"); !errors.Is(err, masterdata.ErrClientNotFound) {
		t.Errorf("GetClientSecret() error = %v, want ErrClientNotFound", err)
	}
	if _, err := svc.GetClient(ctx, testIP); validationError(t, err).Cause() != dto.CauseMandatoryIEIncorrect {
		t.Errorf("GetClient() with an IP error = %v", err)
	}
}

func TestUpdateClient(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	id := seedClient(t, svc, testIP)
	key := masterdata.ClientKey(testIP)

	c, err := svc.UpdateClient(ctx, testActor, id, dto.ClientUpdate{Secret: set("n3w-secret"), Vendor: set("Vendor X")})
	if err != nil {
		t.Fatalf("UpdateClient() error = %v", err)
	}
	if c.Name != "AP-01" || c.Vendor != "Vendor X" || mr.HGet(key, "secret") != "n3w-secret" {
		t.Errorf("UpdateClient() = %+v", c)
	}
	if e := lastAudit(t, buf); e["details"] != `secret: changed, vendor: "" -> "Vendor X"` || e["target_id"] != id {
		t.Errorf("audit = %v", e)
	}
	if strings.Contains(buf.String(), "n3w-secret") {
		t.Error("audit log contains the secret")
	}

	// IP の変更（ID は変わらない）
	c, err = svc.UpdateClient(ctx, testActor, id, dto.ClientUpdate{IP: set(" 10.0.0.5 ")})
	if err != nil {
		t.Fatalf("UpdateClient() IP error = %v", err)
	}
	if c.IP != "10.0.0.5" || strconv.FormatInt(c.ID, 10) != id || mr.Exists(key) || mr.HGet(masterdata.ClientKey("10.0.0.5"), "secret") != "n3w-secret" {
		t.Errorf("after IP change = %+v, keys = %v", c, mr.Keys())
	}
	if e := lastAudit(t, buf); e["details"] != "ip: 192.168.10.1 -> 10.0.0.5" || e["target_key"] != "client:10.0.0.5" {
		t.Errorf("audit = %v", e)
	}

	// 変更後の IP のクライアントが既に存在する
	seedClient(t, svc, "10.0.0.6")
	if _, err := svc.UpdateClient(ctx, testActor, id, dto.ClientUpdate{IP: set("10.0.0.6")}); !errors.Is(err, masterdata.ErrClientExists) {
		t.Errorf("UpdateClient() IP conflict error = %v, want ErrClientExists", err)
	}

	tests := []struct {
		name       string
		id         string
		upd        dto.ClientUpdate
		wantCause  string
		wantParams []string
	}{
		{"bad id", "x", dto.ClientUpdate{Name: set("AP")}, dto.CauseMandatoryIEIncorrect, []string{"clientId"}},
		{"empty patch", id, dto.ClientUpdate{}, dto.CauseMandatoryIEMissing, nil},
		{"null and bad values", id, dto.ClientUpdate{Name: dto.Optional[string]{Set: true, Null: true}, Secret: set(""), IP: set("1.2.3")}, dto.CauseOptionalIEIncorrect, []string{"ip", "secret", "name"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateClient(ctx, testActor, tt.id, tt.upd)
			ve := validationError(t, err)
			if ve.Cause() != tt.wantCause || !slices.Equal(paramNames(ve), tt.wantParams) {
				t.Errorf("cause = %s, params = %v", ve.Cause(), paramNames(ve))
			}
		})
	}

	if _, err := svc.UpdateClient(ctx, testActor, "99", dto.ClientUpdate{Name: set("AP")}); !errors.Is(err, masterdata.ErrClientNotFound) {
		t.Errorf("UpdateClient() error = %v, want ErrClientNotFound", err)
	}
	mr.SetError("forced error")
	if _, err := svc.UpdateClient(ctx, testActor, id, dto.ClientUpdate{Name: set("AP")}); err == nil {
		t.Error("UpdateClient() expected error")
	}
}

func TestDeleteClient(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	id := seedClient(t, svc, testIP)

	if err := svc.DeleteClient(ctx, testActor, id); err != nil {
		t.Fatalf("DeleteClient() error = %v", err)
	}
	if mr.Exists(masterdata.ClientKey(testIP)) || mr.Exists(masterdata.PrefixClientIndex+id) {
		t.Errorf("client still exists: %v", mr.Keys())
	}
	if e := lastAudit(t, buf); e["operation"] != "delete" || e["details"] != `ip=192.168.10.1, name="AP-01", vendor=""` || e["target_id"] != id {
		t.Errorf("audit = %v", e)
	}
	if err := svc.DeleteClient(ctx, testActor, id); !errors.Is(err, masterdata.ErrClientNotFound) {
		t.Errorf("DeleteClient() error = %v, want ErrClientNotFound", err)
	}
	if err := svc.DeleteClient(ctx, testActor, "0"); validationError(t, err).Cause() != dto.CauseMandatoryIEIncorrect {
		t.Errorf("DeleteClient() bad id error = %v", err)
	}
	seedClient(t, svc, "10.0.0.1")
	mr.SetError("forced error")
	if err := svc.DeleteClient(ctx, testActor, "2"); err == nil {
		t.Error("DeleteClient() expected error")
	}
}

func TestListClients(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()
	for _, ip := range []string{"192.168.10.20", "10.0.0.1", "192.168.10.3"} {
		seedClient(t, svc, ip)
	}
	// 解釈できない IP は後ろに置く
	mr.HSet(masterdata.ClientKey("zz"), "secret", "s")
	mr.HSet(masterdata.ClientKey("aa"), "secret", "s")

	clients, err := svc.ListClients(ctx, "")
	if err != nil {
		t.Fatalf("ListClients() error = %v", err)
	}
	var ips []string
	for _, c := range clients {
		ips = append(ips, c.IP)
	}
	// 文字列ではなく数値として比較する（192.168.10.3 が 192.168.10.20 より前）
	if want := []string{"10.0.0.1", "192.168.10.3", "192.168.10.20", "aa", "zz"}; !slices.Equal(ips, want) {
		t.Errorf("ListClients() = %v, want %v", ips, want)
	}

	// IP で絞り込む
	got, err := svc.ListClients(ctx, "10.0.0.1")
	if err != nil || len(got) != 1 || got[0].ID != 2 {
		t.Errorf("ListClients(ip) = %+v, %v", got, err)
	}
	if got, err := svc.ListClients(ctx, "10.9.9.9"); err != nil || len(got) != 0 {
		t.Errorf("ListClients(unknown ip) = %+v, %v", got, err)
	}
	if _, err := svc.ListClients(ctx, "10.0.0"); validationError(t, err).Cause() != dto.CauseInvalidQueryParam {
		t.Errorf("ListClients(bad ip) error = %v", err)
	}

	mr.SetError("forced error")
	if _, err := svc.ListClients(ctx, ""); err == nil {
		t.Error("ListClients() expected error")
	}
	if _, err := svc.ListClients(ctx, "10.0.0.1"); err == nil {
		t.Error("ListClients(ip) expected error")
	}
}

func TestEnsureClientIDs(t *testing.T) {
	svc, mr, _ := newTestService(t)
	mr.HSet(masterdata.ClientKey("10.0.0.1"), "secret", "s", "name", "legacy", "vendor", "")
	if n, err := svc.EnsureClientIDs(context.Background()); err != nil || n != 1 {
		t.Fatalf("EnsureClientIDs() = %d, %v", n, err)
	}
	c, err := svc.GetClient(context.Background(), "1")
	if err != nil || c.IP != "10.0.0.1" {
		t.Errorf("GetClient(1) = %+v, %v", c, err)
	}
}
