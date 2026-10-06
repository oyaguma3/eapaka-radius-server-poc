package masterdata

import (
	"context"
	"errors"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// ErrSubscriberNotFound は加入者が見つからない場合のエラー
var ErrSubscriberNotFound = errors.New("subscriber not found")

// ErrSubscriberExists は同じIMSIの加入者が既に存在する場合のエラー
var ErrSubscriberExists = errors.New("subscriber already exists")

// SubscriberStore は加入者データへのアクセスを提供する。
type SubscriberStore struct {
	client *redis.Client
}

// NewSubscriberStore は新しいSubscriberStoreを生成する。
func NewSubscriberStore(client *redis.Client) *SubscriberStore {
	return &SubscriberStore{client: client}
}

// Get は指定されたIMSIの加入者を取得する。
// Vector APIと互換性のあるHash形式で読み取る。
func (s *SubscriberStore) Get(ctx context.Context, imsi string) (*model.Subscriber, error) {
	key := SubscriberKey(imsi)
	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// キーが存在しない場合、HGetAllは空mapを返す
	if len(result) == 0 {
		return nil, ErrSubscriberNotFound
	}

	return subscriberFromHash(imsi, result), nil
}

// Create は新しい加入者を作成する。
// Vector APIと互換性のあるHash形式で保存する。
// 存在確認と書き込みを1回の操作で行い、既に存在すれば何も書き込まずに ErrSubscriberExists を返す。
func (s *SubscriberStore) Create(ctx context.Context, sub *model.Subscriber) error {
	// created_atが未設定の場合は現在時刻を設定
	createdAt := sub.CreatedAt
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339)
	}

	created, err := runHashScript(ctx, s.client, createHashScript, SubscriberKey(sub.IMSI), map[string]any{
		"ki":         sub.Ki,
		"opc":        sub.OPc,
		"amf":        sub.AMF,
		"sqn":        sub.SQN,
		"created_at": createdAt,
	})
	if err != nil {
		return err
	}
	if !created {
		return ErrSubscriberExists
	}
	return nil
}

// updateSubscriberScript は既存の加入者の ki / opc / amf を更新する。
// ARGV[4] が "1" のときは sqn も更新するが、現在の sqn が ARGV[5]（編集開始時に読んだ値）と
// 一致するときに限る（認証で Vector API が sqn を進めていた場合に巻き戻さないため）。
// 存在チェックと更新を1回で行うので、途中で加入者が削除されても一部のフィールドだけの Hash を作らない。
// 戻り値: 1=更新した、0=加入者が存在しない、-1=sqn が期待値と一致しない（何も更新しない）
var updateSubscriberScript = redis.NewScript(`
if redis.call('EXISTS', KEYS[1]) == 0 then
  return 0
end
if ARGV[4] == '1' then
  if redis.call('HGET', KEYS[1], 'sqn') ~= ARGV[5] then
    return -1
  end
  redis.call('HSET', KEYS[1], 'ki', ARGV[1], 'opc', ARGV[2], 'amf', ARGV[3], 'sqn', ARGV[6])
else
  redis.call('HSET', KEYS[1], 'ki', ARGV[1], 'opc', ARGV[2], 'amf', ARGV[3])
end
return 1
`)

// ErrSQNChanged は、編集中に SQN が他の処理（認証時の Vector API の更新など）で変わっていたことを表す。
var ErrSQNChanged = errors.New("SQN was changed while editing")

// Update は既存の加入者の Ki / OPc / AMF を更新する。SQN は書き換えない。
// SQN は認証のたびに Vector API が進めるため、編集開始時の値で上書きすると巻き戻るおそれがある。
func (s *SubscriberStore) Update(ctx context.Context, sub *model.Subscriber) error {
	return s.update(ctx, sub, false, "")
}

// UpdateWithSQN は既存の加入者の Ki / OPc / AMF と SQN を更新する。
// 現在の SQN が originalSQN（編集開始時に読んだ値）と一致するときだけ更新し、
// 変わっていたときは何も更新せずに ErrSQNChanged を返す。
func (s *SubscriberStore) UpdateWithSQN(ctx context.Context, sub *model.Subscriber, originalSQN string) error {
	return s.update(ctx, sub, true, originalSQN)
}

func (s *SubscriberStore) update(ctx context.Context, sub *model.Subscriber, withSQN bool, originalSQN string) error {
	updateSQN := "0"
	if withSQN {
		updateSQN = "1"
	}

	n, err := updateSubscriberScript.Run(ctx, s.client, []string{SubscriberKey(sub.IMSI)},
		sub.Ki, sub.OPc, sub.AMF, updateSQN, originalSQN, sub.SQN).Int()
	if err != nil {
		return err
	}

	switch n {
	case 0:
		return ErrSubscriberNotFound
	case -1:
		return ErrSQNChanged
	default:
		return nil
	}
}

// SubscriberPatch は加入者の変更内容を表す。nil の項目は変更しない（JSON Merge Patch 用。D-13 §3.1）。
type SubscriberPatch struct {
	Ki  *string
	OPc *string
	AMF *string
	SQN *string // nil なら SQN には触れない（認証で進んだ SQN を巻き戻さない）
}

// IsEmpty は変更する項目がないかを返す。
func (p *SubscriberPatch) IsEmpty() bool {
	return p.Ki == nil && p.OPc == nil && p.AMF == nil && p.SQN == nil
}

// Patch は既存の加入者の、指定した項目だけを書き換える。
// SQN を指定した場合は比較せずにそのまま書き換える（D-13 §3.1。Admin TUI の UpdateWithSQN とは異なる）。
// 存在確認と書き込みを1回の操作で行い、加入者が存在しなければ ErrSubscriberNotFound を返す。
// 変更する項目がなければ、存在だけを確認する。
func (s *SubscriberStore) Patch(ctx context.Context, imsi string, patch *SubscriberPatch) error {
	fields := map[string]any{}
	for name, v := range map[string]*string{"ki": patch.Ki, "opc": patch.OPc, "amf": patch.AMF, "sqn": patch.SQN} {
		if v != nil {
			fields[name] = *v
		}
	}
	if len(fields) == 0 {
		exists, err := s.Exists(ctx, imsi)
		if err != nil {
			return err
		}
		if !exists {
			return ErrSubscriberNotFound
		}
		return nil
	}

	updated, err := runHashScript(ctx, s.client, updateHashScript, SubscriberKey(imsi), fields)
	if err != nil {
		return err
	}
	if !updated {
		return ErrSubscriberNotFound
	}
	return nil
}

// Delete は加入者を削除する。
func (s *SubscriberStore) Delete(ctx context.Context, imsi string) error {
	key := SubscriberKey(imsi)

	result, err := s.client.Del(ctx, key).Result()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrSubscriberNotFound
	}
	return nil
}

// List は全加入者のリストを取得する（SCAN使用）。
func (s *SubscriberStore) List(ctx context.Context) ([]*model.Subscriber, error) {
	var subscribers []*model.Subscriber
	var keys []string

	// SCANで全キーを取得
	iter := s.client.Scan(ctx, 0, PrefixSubscriber+"*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return subscribers, nil
	}

	// Pipelineで一括取得（HGETALL）
	pipe := s.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	for i, cmd := range cmds {
		result, err := cmd.Result()
		if err != nil {
			continue
		}

		// 空の結果はスキップ
		if len(result) == 0 {
			continue
		}

		// キーからIMSIを抽出
		imsi := keys[i][len(PrefixSubscriber):]
		subscribers = append(subscribers, subscriberFromHash(imsi, result))
	}

	return subscribers, nil
}

// Count は加入者の総数を返す。
func (s *SubscriberStore) Count(ctx context.Context) (int64, error) {
	var count int64

	iter := s.client.Scan(ctx, 0, PrefixSubscriber+"*", 100).Iterator()
	for iter.Next(ctx) {
		count++
	}
	if err := iter.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

// Exists は指定されたIMSIの加入者が存在するか確認する。
func (s *SubscriberStore) Exists(ctx context.Context, imsi string) (bool, error) {
	key := SubscriberKey(imsi)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// BulkCreate は複数の加入者を一括で作成する（TxPipeline使用）。
func (s *SubscriberStore) BulkCreate(ctx context.Context, subscribers []*model.Subscriber) error {
	if len(subscribers) == 0 {
		return nil
	}

	pipe := s.client.TxPipeline()
	for _, sub := range subscribers {
		key := SubscriberKey(sub.IMSI)

		createdAt := sub.CreatedAt
		if createdAt == "" {
			createdAt = time.Now().UTC().Format(time.RFC3339)
		}

		pipe.HSet(ctx, key, map[string]any{
			"ki":         sub.Ki,
			"opc":        sub.OPc,
			"amf":        sub.AMF,
			"sqn":        sub.SQN,
			"created_at": createdAt,
		})
	}

	_, err := pipe.Exec(ctx)
	return err
}

// subscriberFromHash はHashマップからSubscriberを構築する。
func subscriberFromHash(imsi string, fields map[string]string) *model.Subscriber {
	return &model.Subscriber{
		IMSI:      imsi,
		Ki:        fields["ki"],
		OPc:       fields["opc"],
		AMF:       fields["amf"],
		SQN:       fields["sqn"],
		CreatedAt: fields["created_at"],
	}
}
