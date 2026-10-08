package service

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/validation"
)

// errNonCanonicalIP は先頭に 0 のある数値等、Auth Server が送信元IPから引くキーと一致しない表記を表す。
var errNonCanonicalIP = errors.New("must be a valid IPv4 address without leading zeros")

// clientIDPattern はパスの RADIUSクライアントのID（1 以上の整数）の表記。
var clientIDPattern = regexp.MustCompile(`^[1-9][0-9]{0,17}$`)

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

// parseClientID はパスの RADIUSクライアントのIDを検証して数値にする。
func parseClientID(s string) (int64, error) {
	if !clientIDPattern.MatchString(s) {
		return 0, &ValidationError{Mandatory: []dto.InvalidParam{{Param: "clientId", Reason: "must be a positive integer"}}}
	}
	id, _ := strconv.ParseInt(s, 10, 64)
	return id, nil
}

// clientEntry は RADIUSクライアントの監査ログの共通部分を返す。
func clientEntry(op audit.Operation, c *model.RadiusClient) audit.Entry {
	return audit.Entry{
		Operation:  op,
		TargetType: audit.TargetClient,
		TargetKey:  masterdata.ClientKey(c.IP),
		TargetID:   strconv.FormatInt(c.ID, 10),
	}
}

// EnsureClientIDs は、ID を持たない RADIUSクライアント（ID の導入前に登録されたもの）に ID を採番し、件数を返す。
func (s *Service) EnsureClientIDs(ctx context.Context) (int, error) {
	return s.clients.EnsureIDs(ctx)
}

// ListClients は RADIUSクライアントの全件を IP アドレスの順（数値として比較）で返す。
// ipFilter を指定した場合は、その IP のクライアントだけ（0 件または 1 件）を返す。
func (s *Service) ListClients(ctx context.Context, ipFilter string) ([]*model.RadiusClient, error) {
	if ipFilter != "" {
		if err := validateIP(ipFilter); err != nil {
			return nil, &ValidationError{Query: []dto.InvalidParam{{Param: "ip", Reason: reason(err)}}}
		}
		c, err := s.clients.Get(ctx, ipFilter)
		if errors.Is(err, masterdata.ErrClientNotFound) {
			return []*model.RadiusClient{}, nil
		}
		if err != nil {
			return nil, err
		}
		return []*model.RadiusClient{c}, nil
	}

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

// CreateClient は RADIUSクライアントを登録し、ID を採番する。同じ IP が既に存在すれば masterdata.ErrClientExists を返す。
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

	e := clientEntry(audit.OpCreate, c)
	e.Details = clientState(c)
	s.audit.Record(ctx, actor, e)
	return c, nil
}

// GetClient は RADIUSクライアントを返す。存在しなければ masterdata.ErrClientNotFound を返す。
func (s *Service) GetClient(ctx context.Context, id string) (*model.RadiusClient, error) {
	n, err := parseClientID(id)
	if err != nil {
		return nil, err
	}
	return s.clients.GetByID(ctx, n)
}

// GetClientSecret は RADIUSクライアントの共有シークレットを返し、読み出したことを監査ログに記録する。
func (s *Service) GetClientSecret(ctx context.Context, actor audit.Actor, id string) (*model.RadiusClient, error) {
	c, err := s.GetClient(ctx, id)
	if err != nil {
		return nil, err
	}
	e := clientEntry(audit.OpRead, c)
	e.Details = "secret"
	s.audit.Record(ctx, actor, e)
	return c, nil
}

// UpdateClient は RADIUSクライアントの、指定した項目だけを書き換える（JSON Merge Patch）。
// ip を指定した場合は IP を変える（ID は変わらない）。変更後の IP のクライアントが既に存在すれば masterdata.ErrClientExists を返す。
func (s *Service) UpdateClient(ctx context.Context, actor audit.Actor, id string, upd dto.ClientUpdate) (*model.RadiusClient, error) {
	n, err := parseClientID(id)
	if err != nil {
		return nil, err
	}
	if upd.IsEmpty() {
		return nil, emptyPatch()
	}

	norm := validation.NormalizeClientInput(&validation.ClientInput{
		IP: upd.IP.Value, Secret: upd.Secret.Value, Name: upd.Name.Value, Vendor: upd.Vendor.Value,
	})
	var v ValidationError
	patch := &masterdata.ClientPatch{
		IP:     v.patchField("ip", upd.IP, norm.IP, validateIP),
		Secret: v.patchField("secret", upd.Secret, norm.Secret, validation.ValidateSecret),
		Name:   v.patchField("name", upd.Name, norm.Name, validation.ValidateClientName),
		Vendor: v.patchField("vendor", upd.Vendor, norm.Vendor, validation.ValidateVendor),
	}
	if err := v.orNil(); err != nil {
		return nil, err
	}

	// 監査ログに変更前の値を残すため、また ID から現在の IP を引くため、先に読む（存在しなければここで 404 になる）
	before, err := s.clients.GetByID(ctx, n)
	if err != nil {
		return nil, err
	}
	if err := s.clients.Patch(ctx, before.IP, patch); err != nil {
		return nil, err
	}
	after, err := s.clients.GetByID(ctx, n)
	if err != nil {
		return nil, err
	}

	var c changes
	if patch.IP != nil && *patch.IP != before.IP {
		c.value("ip", before.IP, patch.IP)
	}
	c.secret("secret", before.Secret, patch.Secret)
	c.value("name", fmt.Sprintf("%q", before.Name), quote(patch.Name))
	c.value("vendor", fmt.Sprintf("%q", before.Vendor), quote(patch.Vendor))
	e := clientEntry(audit.OpUpdate, after)
	e.Details = c.String()
	s.audit.Record(ctx, actor, e)
	return after, nil
}

// DeleteClient は RADIUSクライアントを削除する。
func (s *Service) DeleteClient(ctx context.Context, actor audit.Actor, id string) error {
	n, err := parseClientID(id)
	if err != nil {
		return err
	}
	// ID から IP を引く（存在しなければここで 404 になる）。監査ログには削除前の値を残す
	before, err := s.clients.GetByID(ctx, n)
	if err != nil {
		return err
	}
	if err := s.clients.Delete(ctx, before.IP); err != nil {
		return err
	}

	e := clientEntry(audit.OpDelete, before)
	e.Details = clientState(before)
	s.audit.Record(ctx, actor, e)
	return nil
}

// clientState は監査ログに残す RADIUSクライアントの状態（共有シークレットは含めない）。
func clientState(c *model.RadiusClient) string {
	return fmt.Sprintf("ip=%s, name=%q, vendor=%q", c.IP, c.Name, c.Vendor)
}

// quote はポインターの値を引用符で囲む。nil なら nil を返す。
func quote(p *string) *string {
	if p == nil {
		return nil
	}
	q := fmt.Sprintf("%q", *p)
	return &q
}
