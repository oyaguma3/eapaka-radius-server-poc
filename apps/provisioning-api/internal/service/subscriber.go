package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/validation"
)

// 加入者の作成時の既定値（D-13 §3.1。Admin TUI と同じ）
const (
	defaultAMF = "8000"
	defaultSQN = "000000000000"
)

// ListSubscribers は加入者の一覧を IMSI の昇順で返す。
func (s *Service) ListSubscribers(ctx context.Context, q dto.ListQuery) (*masterdata.SubscriberPage, error) {
	prefix, cursor, limit, err := parseListQuery(q)
	if err != nil {
		return nil, err
	}
	return s.subs.ListPage(ctx, prefix, cursor, limit)
}

// CreateSubscriber は加入者を登録する。既に存在すれば masterdata.ErrSubscriberExists を返す。
func (s *Service) CreateSubscriber(ctx context.Context, actor audit.Actor, req dto.SubscriberCreate) (*model.Subscriber, error) {
	in := validation.NormalizeSubscriberInput(&validation.SubscriberInput{
		IMSI: deref(req.IMSI, ""),
		Ki:   deref(req.Ki, ""),
		OPc:  deref(req.OPc, ""),
		AMF:  deref(req.AMF, defaultAMF),
		SQN:  deref(req.SQN, defaultSQN),
	})

	var v ValidationError
	v.required("imsi", req.IMSI != nil, validation.ValidateIMSI(in.IMSI))
	v.required("ki", req.Ki != nil, validation.ValidateKi(in.Ki))
	v.required("opc", req.OPc != nil, validation.ValidateOPc(in.OPc))
	v.optional("amf", validation.ValidateAMF(in.AMF))
	v.optional("sqn", validation.ValidateSQN(in.SQN))
	if err := v.orNil(); err != nil {
		return nil, err
	}

	sub := &model.Subscriber{
		IMSI:      in.IMSI,
		Ki:        in.Ki,
		OPc:       in.OPc,
		AMF:       in.AMF,
		SQN:       in.SQN,
		CreatedAt: s.now().UTC().Format(time.RFC3339),
	}
	if err := s.subs.Create(ctx, sub); err != nil {
		return nil, err
	}

	s.audit.Record(ctx, actor, audit.Entry{
		Operation:  audit.OpCreate,
		TargetType: audit.TargetSubscriber,
		TargetKey:  masterdata.SubscriberKey(sub.IMSI),
		TargetIMSI: sub.IMSI,
		Details:    subscriberState(sub),
	})
	return sub, nil
}

// GetSubscriber は加入者を返す。存在しなければ masterdata.ErrSubscriberNotFound を返す。
func (s *Service) GetSubscriber(ctx context.Context, imsi string) (*model.Subscriber, error) {
	if err := checkIMSI(imsi); err != nil {
		return nil, err
	}
	return s.subs.Get(ctx, imsi)
}

// GetSubscriberKeys は加入者の Ki と OPc を返し、読み出したことを監査ログに記録する。
func (s *Service) GetSubscriberKeys(ctx context.Context, actor audit.Actor, imsi string) (*model.Subscriber, error) {
	sub, err := s.GetSubscriber(ctx, imsi)
	if err != nil {
		return nil, err
	}
	s.audit.Record(ctx, actor, audit.Entry{
		Operation:  audit.OpRead,
		TargetType: audit.TargetSubscriber,
		TargetKey:  masterdata.SubscriberKey(imsi),
		TargetIMSI: imsi,
		Details:    "ki,opc",
	})
	return sub, nil
}

// UpdateSubscriber は加入者の、指定した項目だけを書き換える（JSON Merge Patch）。
// sqn を指定しなければ SQN には触れない。指定した場合は、そのまま書き換える（D-13 §3.1）。
func (s *Service) UpdateSubscriber(ctx context.Context, actor audit.Actor, imsi string, upd dto.SubscriberUpdate) (*model.Subscriber, error) {
	if err := checkIMSI(imsi); err != nil {
		return nil, err
	}
	if upd.IsEmpty() {
		return nil, emptyPatch()
	}

	n := validation.NormalizeSubscriberInput(&validation.SubscriberInput{
		Ki: upd.Ki.Value, OPc: upd.OPc.Value, AMF: upd.AMF.Value, SQN: upd.SQN.Value,
	})
	var v ValidationError
	patch := &masterdata.SubscriberPatch{
		Ki:  v.patchField("ki", upd.Ki, n.Ki, validation.ValidateKi),
		OPc: v.patchField("opc", upd.OPc, n.OPc, validation.ValidateOPc),
		AMF: v.patchField("amf", upd.AMF, n.AMF, validation.ValidateAMF),
		SQN: v.patchField("sqn", upd.SQN, n.SQN, validation.ValidateSQN),
	}
	if err := v.orNil(); err != nil {
		return nil, err
	}

	// 監査ログに変更前の値を残すため、先に読む（存在しなければここで 404 になる）
	before, err := s.subs.Get(ctx, imsi)
	if err != nil {
		return nil, err
	}
	if err := s.subs.Patch(ctx, imsi, patch); err != nil {
		return nil, err
	}
	after, err := s.subs.Get(ctx, imsi)
	if err != nil {
		return nil, err
	}

	var c changes
	c.secret("ki", before.Ki, patch.Ki)
	c.secret("opc", before.OPc, patch.OPc)
	c.value("amf", strings.ToLower(before.AMF), lower(patch.AMF))
	c.value("sqn", strings.ToLower(before.SQN), lower(patch.SQN))
	s.audit.Record(ctx, actor, audit.Entry{
		Operation:  audit.OpUpdate,
		TargetType: audit.TargetSubscriber,
		TargetKey:  masterdata.SubscriberKey(imsi),
		TargetIMSI: imsi,
		Details:    c.String(),
	})
	return after, nil
}

// DeleteSubscriber は加入者（sub:{IMSI}）だけを削除する。認可ポリシー等は削除しない（D-13 §3.1）。
// 削除時点の SQN と AMF を監査ログに記録する。
func (s *Service) DeleteSubscriber(ctx context.Context, actor audit.Actor, imsi string) error {
	if err := checkIMSI(imsi); err != nil {
		return err
	}
	// 監査ログ用に削除前の値を読む。読めなくても削除は行う（存在しなければ Delete が 404 を返す）
	before, _ := s.subs.Get(ctx, imsi)
	if err := s.subs.Delete(ctx, imsi); err != nil {
		return err
	}

	e := audit.Entry{
		Operation:  audit.OpDelete,
		TargetType: audit.TargetSubscriber,
		TargetKey:  masterdata.SubscriberKey(imsi),
		TargetIMSI: imsi,
	}
	if before != nil {
		e.Details = subscriberState(before)
	}
	s.audit.Record(ctx, actor, e)
	return nil
}

// subscriberState は監査ログに残す加入者の状態（Ki と OPc は含めない）。
func subscriberState(sub *model.Subscriber) string {
	return fmt.Sprintf("amf=%s, sqn=%s", strings.ToLower(sub.AMF), strings.ToLower(sub.SQN))
}

// lower はポインターの値を小文字にする。nil なら nil を返す。
func lower(p *string) *string {
	if p == nil {
		return nil
	}
	l := strings.ToLower(*p)
	return &l
}
