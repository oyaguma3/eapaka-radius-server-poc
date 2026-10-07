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

const testSecret = "c2VjcmV0LWV4YW1wbGU"

// seedClient は RADIUSクライアントを登録する。
func seedClient(t *testing.T, svc *Service, ip string) {
	t.Helper()
	_, err := svc.CreateClient(context.Background(), testActor, dto.ClientCreate{IP: ptr(ip), Secret: ptr(testSecret), Name: ptr("AP-01")})
	if err != nil {
		t.Fatalf("CreateClient(%s) error = %v", ip, err)
	}
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

func TestCreateClient(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()

	c, err := svc.CreateClient(ctx, testActor, dto.ClientCreate{IP: ptr(testIP), Secret: ptr(testSecret), Name: ptr("AP-01"), Vendor: ptr("generic")})
	if err != nil {
		t.Fatalf("CreateClient() error = %v", err)
	}
	key := masterdata.ClientKey(testIP)
	if mr.HGet(key, "secret") != testSecret || mr.HGet(key, "name") != "AP-01" || mr.HGet(key, "vendor") != "generic" || c.IP != testIP {
		t.Errorf("stored = %v", mr.Keys())
	}
	e := lastAudit(t, buf)
	if e["operation"] != "create" || e["target_type"] != "client" || e["target_key"] != key || e["details"] != `name="AP-01", vendor="generic"` {
		t.Errorf("audit = %v", e)
	}
	if _, ok := e["target_imsi"]; ok {
		t.Error("client audit has target_imsi")
	}
	if strings.Contains(buf.String(), testSecret) {
		t.Errorf("audit log contains the secret: %s", buf.String())
	}

	// vendor は省略できる（空文字）
	if _, err := svc.CreateClient(ctx, testActor, dto.ClientCreate{IP: ptr("10.0.0.1"), Secret: ptr("s"), Name: ptr("AP-02")}); err != nil {
		t.Fatalf("CreateClient() without vendor error = %v", err)
	}
	if got := mr.HGet(masterdata.ClientKey("10.0.0.1"), "vendor"); got != "" {
		t.Errorf("vendor = %q", got)
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
	seedClient(t, svc, testIP)
	buf.Reset()

	c, err := svc.GetClient(ctx, testIP)
	if err != nil || c.Name != "AP-01" {
		t.Fatalf("GetClient() = %+v, %v", c, err)
	}
	if buf.Len() != 0 {
		t.Errorf("GetClient() wrote an audit log")
	}
	c, err = svc.GetClientSecret(ctx, testActor, testIP)
	if err != nil || c.Secret != testSecret {
		t.Fatalf("GetClientSecret() = %+v, %v", c, err)
	}
	if e := lastAudit(t, buf); e["operation"] != "read" || e["details"] != "secret" || e["msg"] != "client secret read" {
		t.Errorf("audit = %v", e)
	}
	if _, err := svc.GetClientSecret(ctx, testActor, "10.0.0.9"); !errors.Is(err, masterdata.ErrClientNotFound) {
		t.Errorf("GetClientSecret() error = %v, want ErrClientNotFound", err)
	}
	if _, err := svc.GetClient(ctx, "10.0.0"); validationError(t, err).Cause() != dto.CauseMandatoryIEIncorrect {
		t.Errorf("GetClient() bad ip error = %v", err)
	}
}

func TestUpdateClient(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	seedClient(t, svc, testIP)
	key := masterdata.ClientKey(testIP)

	c, err := svc.UpdateClient(ctx, testActor, testIP, dto.ClientUpdate{Secret: set("n3w-secret"), Vendor: set("Vendor X")})
	if err != nil {
		t.Fatalf("UpdateClient() error = %v", err)
	}
	if c.Name != "AP-01" || c.Vendor != "Vendor X" || mr.HGet(key, "secret") != "n3w-secret" {
		t.Errorf("UpdateClient() = %+v", c)
	}
	if e := lastAudit(t, buf); e["details"] != `secret: changed, vendor: "" -> "Vendor X"` {
		t.Errorf("audit details = %v", e["details"])
	}
	if strings.Contains(buf.String(), "n3w-secret") {
		t.Error("audit log contains the secret")
	}

	tests := []struct {
		name       string
		ip         string
		upd        dto.ClientUpdate
		wantCause  string
		wantParams []string
	}{
		{"bad ip", "x", dto.ClientUpdate{Name: set("AP")}, dto.CauseMandatoryIEIncorrect, []string{"ip"}},
		{"empty patch", testIP, dto.ClientUpdate{}, dto.CauseMandatoryIEMissing, nil},
		{"null and bad values", testIP, dto.ClientUpdate{Name: dto.Optional[string]{Set: true, Null: true}, Secret: set("")}, dto.CauseOptionalIEIncorrect, []string{"secret", "name"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.UpdateClient(ctx, testActor, tt.ip, tt.upd)
			ve := validationError(t, err)
			if ve.Cause() != tt.wantCause || !slices.Equal(paramNames(ve), tt.wantParams) {
				t.Errorf("cause = %s, params = %v", ve.Cause(), paramNames(ve))
			}
		})
	}

	if _, err := svc.UpdateClient(ctx, testActor, "10.0.0.9", dto.ClientUpdate{Name: set("AP")}); !errors.Is(err, masterdata.ErrClientNotFound) {
		t.Errorf("UpdateClient() error = %v, want ErrClientNotFound", err)
	}
	mr.SetError("forced error")
	if _, err := svc.UpdateClient(ctx, testActor, testIP, dto.ClientUpdate{Name: set("AP")}); err == nil {
		t.Error("UpdateClient() expected error")
	}
}

func TestDeleteClient(t *testing.T) {
	svc, mr, buf := newTestService(t)
	ctx := context.Background()
	seedClient(t, svc, testIP)

	if err := svc.DeleteClient(ctx, testActor, testIP); err != nil {
		t.Fatalf("DeleteClient() error = %v", err)
	}
	if mr.Exists(masterdata.ClientKey(testIP)) {
		t.Error("client still exists")
	}
	if e := lastAudit(t, buf); e["operation"] != "delete" || e["details"] != `name="AP-01", vendor=""` {
		t.Errorf("audit = %v", e)
	}
	if err := svc.DeleteClient(ctx, testActor, testIP); !errors.Is(err, masterdata.ErrClientNotFound) {
		t.Errorf("DeleteClient() error = %v, want ErrClientNotFound", err)
	}
	if err := svc.DeleteClient(ctx, testActor, "1.2.3"); validationError(t, err).Cause() != dto.CauseMandatoryIEIncorrect {
		t.Errorf("DeleteClient() bad ip error = %v", err)
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

	clients, err := svc.ListClients(ctx)
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

	mr.SetError("forced error")
	if _, err := svc.ListClients(ctx); err == nil {
		t.Error("ListClients() expected error")
	}
}
