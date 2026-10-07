package service

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/validation"
)

// errNonCanonicalIP は先頭に 0 のある数値等、Auth Server が送信元IPから引くキーと一致しない表記を表す。
var errNonCanonicalIP = errors.New("must be a valid IPv4 address without leading zeros")

// validateIP は IPv4 アドレスを検証する（pkg/validation の規則に加え、表記が一意であること）。
// Auth Server は送信元IPの文字列表記で client:{IP} を引くため、"192.168.010.1" 等は受け付けない。
func validateIP(ip string) error {
	if err := validation.ValidateIPv4(ip); err != nil {
		return err
	}
	addr, err := netip.ParseAddr(ip)
	if err != nil || addr.String() != ip {
		return errNonCanonicalIP
	}
	return nil
}

// checkIP はパスの IP アドレスを検証する。
func checkIP(ip string) error {
	if err := validateIP(ip); err != nil {
		return &ValidationError{Mandatory: []dto.InvalidParam{{Param: "ip", Reason: reason(err)}}}
	}
	return nil
}

// ListClients は RADIUSクライアントの全件を IP アドレスの順（数値として比較）で返す。
func (s *Service) ListClients(ctx context.Context) ([]*model.RadiusClient, error) {
	clients, err := s.clients.List(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(clients, func(a, b *model.RadiusClient) int {
		aa, aErr := netip.ParseAddr(a.IP)
		ba, bErr := netip.ParseAddr(b.IP)
		switch {
		case aErr == nil && bErr == nil:
			return aa.Compare(ba)
		case aErr == nil:
			return -1 // 解釈できない IP（Admin TUI 以外で書かれたもの等）は後ろに置く
		case bErr == nil:
			return 1
		default:
			return strings.Compare(a.IP, b.IP)
		}
	})
	return clients, nil
}

// CreateClient は RADIUSクライアントを登録する。既に存在すれば masterdata.ErrClientExists を返す。
func (s *Service) CreateClient(ctx context.Context, actor audit.Actor, req dto.ClientCreate) (*model.RadiusClient, error) {
	in := validation.NormalizeClientInput(&validation.ClientInput{
		IP:     deref(req.IP, ""),
		Secret: deref(req.Secret, ""),
		Name:   deref(req.Name, ""),
		Vendor: deref(req.Vendor, ""),
	})

	var v ValidationError
	v.required("ip", req.IP != nil, validateIP(in.IP))
	v.required("secret", req.Secret != nil, validation.ValidateSecret(in.Secret))
	v.required("name", req.Name != nil, validation.ValidateClientName(in.Name))
	v.optional("vendor", validation.ValidateVendor(in.Vendor))
	if err := v.orNil(); err != nil {
		return nil, err
	}

	c := &model.RadiusClient{IP: in.IP, Secret: in.Secret, Name: in.Name, Vendor: in.Vendor}
	if err := s.clients.Create(ctx, c); err != nil {
		return nil, err
	}

	s.audit.Record(actor, audit.Entry{
		Operation:  audit.OpCreate,
		TargetType: audit.TargetClient,
		TargetKey:  masterdata.ClientKey(c.IP),
		Details:    clientState(c),
	})
	return c, nil
}

// GetClient は RADIUSクライアントを返す。存在しなければ masterdata.ErrClientNotFound を返す。
func (s *Service) GetClient(ctx context.Context, ip string) (*model.RadiusClient, error) {
	if err := checkIP(ip); err != nil {
		return nil, err
	}
	return s.clients.Get(ctx, ip)
}

// GetClientSecret は RADIUSクライアントの共有シークレットを返し、読み出したことを監査ログに記録する。
func (s *Service) GetClientSecret(ctx context.Context, actor audit.Actor, ip string) (*model.RadiusClient, error) {
	c, err := s.GetClient(ctx, ip)
	if err != nil {
		return nil, err
	}
	s.audit.Record(actor, audit.Entry{
		Operation:  audit.OpRead,
		TargetType: audit.TargetClient,
		TargetKey:  masterdata.ClientKey(ip),
		Details:    "secret",
	})
	return c, nil
}

// UpdateClient は RADIUSクライアントの、指定した項目だけを書き換える（JSON Merge Patch）。
func (s *Service) UpdateClient(ctx context.Context, actor audit.Actor, ip string, upd dto.ClientUpdate) (*model.RadiusClient, error) {
	if err := checkIP(ip); err != nil {
		return nil, err
	}
	if upd.IsEmpty() {
		return nil, emptyPatch()
	}

	n := validation.NormalizeClientInput(&validation.ClientInput{
		Secret: upd.Secret.Value, Name: upd.Name.Value, Vendor: upd.Vendor.Value,
	})
	var v ValidationError
	patch := &masterdata.ClientPatch{
		Secret: v.patchField("secret", upd.Secret, n.Secret, validation.ValidateSecret),
		Name:   v.patchField("name", upd.Name, n.Name, validation.ValidateClientName),
		Vendor: v.patchField("vendor", upd.Vendor, n.Vendor, validation.ValidateVendor),
	}
	if err := v.orNil(); err != nil {
		return nil, err
	}

	// 監査ログに変更前の値を残すため、先に読む（存在しなければここで 404 になる）
	before, err := s.clients.Get(ctx, ip)
	if err != nil {
		return nil, err
	}
	if err := s.clients.Patch(ctx, ip, patch); err != nil {
		return nil, err
	}
	after, err := s.clients.Get(ctx, ip)
	if err != nil {
		return nil, err
	}

	var c changes
	c.secret("secret", before.Secret, patch.Secret)
	c.value("name", fmt.Sprintf("%q", before.Name), quote(patch.Name))
	c.value("vendor", fmt.Sprintf("%q", before.Vendor), quote(patch.Vendor))
	s.audit.Record(actor, audit.Entry{
		Operation:  audit.OpUpdate,
		TargetType: audit.TargetClient,
		TargetKey:  masterdata.ClientKey(ip),
		Details:    c.String(),
	})
	return after, nil
}

// DeleteClient は RADIUSクライアントを削除する。
func (s *Service) DeleteClient(ctx context.Context, actor audit.Actor, ip string) error {
	if err := checkIP(ip); err != nil {
		return err
	}
	// 監査ログ用に削除前の値を読む。読めなくても削除は行う（存在しなければ Delete が 404 を返す）
	before, _ := s.clients.Get(ctx, ip)
	if err := s.clients.Delete(ctx, ip); err != nil {
		return err
	}

	e := audit.Entry{
		Operation:  audit.OpDelete,
		TargetType: audit.TargetClient,
		TargetKey:  masterdata.ClientKey(ip),
	}
	if before != nil {
		e.Details = clientState(before)
	}
	s.audit.Record(actor, e)
	return nil
}

// clientState は監査ログに残す RADIUSクライアントの状態（共有シークレットは含めない）。
func clientState(c *model.RadiusClient) string {
	return fmt.Sprintf("name=%q, vendor=%q", c.Name, c.Vendor)
}

// quote はポインターの値を引用符で囲む。nil なら nil を返す。
func quote(p *string) *string {
	if p == nil {
		return nil
	}
	q := fmt.Sprintf("%q", *p)
	return &q
}
