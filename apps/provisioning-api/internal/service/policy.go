package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/validation"
)

// vlanIDPattern は VLAN ID の表記（数字だけ。OpenAPI 定義の pattern と同じ）。
// pkg/validation.ValidateVlanID は strconv.Atoi で解釈するため "+5" 等も通るので、表記をここで絞る。
var vlanIDPattern = regexp.MustCompile(`^[0-9]{0,4}$`)

// errVlanIDFormat は VLAN ID が数字だけでないことを表す。
var errVlanIDFormat = errors.New("must be a number from 0 to 4094 written in digits only")

// validateVlanID は VLAN ID を検証する（空文字は未設定）。
func validateVlanID(vlanID string) error {
	if !vlanIDPattern.MatchString(vlanID) {
		return errVlanIDFormat
	}
	return validation.ValidateVlanID(vlanID)
}

// ListPolicies は認可ポリシーの一覧を IMSI の昇順で返す。
func (s *Service) ListPolicies(ctx context.Context, q dto.ListQuery) (*masterdata.PolicyPage, error) {
	prefix, cursor, limit, err := parseListQuery(q)
	if err != nil {
		return nil, err
	}
	return s.policies.ListPage(ctx, prefix, cursor, limit)
}

// GetPolicy は認可ポリシーを返す。存在しなければ masterdata.ErrPolicyNotFound を返す。
func (s *Service) GetPolicy(ctx context.Context, imsi string) (*model.Policy, error) {
	if err := checkIMSI(imsi); err != nil {
		return nil, err
	}
	return s.policies.Get(ctx, imsi)
}

// PutPolicy は認可ポリシー全体を置き換える。存在しなければ作成し、created に true を返す（D-13 §3.3）。
// 加入者（sub:{IMSI}）の有無は確認しない。
func (s *Service) PutPolicy(ctx context.Context, actor audit.Actor, imsi string, req dto.PolicyPut) (policy *model.Policy, created bool, err error) {
	if err := checkIMSI(imsi); err != nil {
		return nil, false, err
	}

	in := &validation.PolicyInput{IMSI: imsi, Default: deref(req.Default, ""), Rules: make([]model.PolicyRule, len(req.Rules))}
	for i, r := range req.Rules {
		in.Rules[i] = model.PolicyRule{NasID: deref(r.NasID, ""), AllowedSSIDs: r.AllowedSSIDs, VlanID: deref(r.VlanID, "")}
		if r.SessionTimeout != nil {
			in.Rules[i].SessionTimeout = *r.SessionTimeout
		}
	}
	n := validation.NormalizePolicyInput(in)

	var v ValidationError
	v.required("default", req.Default != nil, validation.ValidateDefaultAction(n.Default))
	if req.Rules == nil {
		v.required("rules", false, nil)
	}
	for i, r := range req.Rules {
		p := fmt.Sprintf("rules[%d]", i)
		nr := n.Rules[i]
		v.required(p+".nasId", r.NasID != nil, validation.ValidateNasID(nr.NasID))
		switch {
		case r.AllowedSSIDs == nil:
			v.required(p+".allowedSsids", false, nil)
		case len(r.AllowedSSIDs) == 0:
			v.required(p+".allowedSsids", true, validation.ValidateAllowedSSIDs(nr.AllowedSSIDs))
		default:
			for j, ssid := range nr.AllowedSSIDs {
				v.required(fmt.Sprintf("%s.allowedSsids[%d]", p, j), true, validation.ValidateSSID(ssid))
			}
		}
		v.optional(p+".vlanId", validateVlanID(nr.VlanID))
		v.optional(p+".sessionTimeout", validation.ValidateSessionTimeout(nr.SessionTimeout))
	}
	if err := v.orNil(); err != nil {
		return nil, false, err
	}

	policy = &model.Policy{IMSI: imsi, Default: n.Default, Rules: n.Rules}
	// 監査ログに変更前の値を残すため、先に読む。読めなくても（rules が壊れている等）置き換えは行う
	before, _ := s.policies.Get(ctx, imsi)
	if created, err = s.policies.Put(ctx, policy); err != nil {
		return nil, false, err
	}

	e := audit.Entry{
		Operation:  audit.OpUpdate,
		TargetType: audit.TargetPolicy,
		TargetKey:  masterdata.PolicyKey(imsi),
		TargetIMSI: imsi,
	}
	if created || before == nil {
		if created {
			e.Operation = audit.OpCreate
		}
		e.Details = policyState(policy)
	} else {
		e.Details = policyChanges(before, policy)
	}
	s.audit.Record(ctx, actor, e)
	return policy, created, nil
}

// DeletePolicy は認可ポリシーを削除する。
func (s *Service) DeletePolicy(ctx context.Context, actor audit.Actor, imsi string) error {
	if err := checkIMSI(imsi); err != nil {
		return err
	}
	// 監査ログ用に削除前の値を読む。読めなくても削除は行う（存在しなければ Delete が 404 を返す）
	before, _ := s.policies.Get(ctx, imsi)
	if err := s.policies.Delete(ctx, imsi); err != nil {
		return err
	}

	e := audit.Entry{
		Operation:  audit.OpDelete,
		TargetType: audit.TargetPolicy,
		TargetKey:  masterdata.PolicyKey(imsi),
		TargetIMSI: imsi,
	}
	if before != nil {
		e.Details = policyState(before)
	}
	s.audit.Record(ctx, actor, e)
	return nil
}

// policyState は監査ログに残す認可ポリシーの状態（default とルールの件数）。
func policyState(p *model.Policy) string {
	return fmt.Sprintf("default=%s, rules=%d", p.Default, len(p.Rules))
}

// policyChanges は監査ログに残す認可ポリシーの変更内容。
// default は変更前後の値を、rules は内容が変わったかと件数の変化を記録する。
func policyChanges(before, after *model.Policy) string {
	var c changes
	c.value("default", before.Default, &after.Default)
	state := "unchanged"
	if !sameRules(before.Rules, after.Rules) {
		state = "changed"
	}
	c = append(c, fmt.Sprintf("rules: %s (%d -> %d)", state, len(before.Rules), len(after.Rules)))
	return c.String()
}

// sameRules はルールの並びが同じかを返す（空の配列と nil は同じとみなす）。
func sameRules(a, b []model.PolicyRule) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
