package store

import (
	"context"
	"errors"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// ErrSubscriberNotFound は加入者が見つからない場合のエラー
var ErrSubscriberNotFound = errors.New("subscriber not found")

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
func (s *SubscriberStore) Create(ctx context.Context, sub *model.Subscriber) error {
	key := SubscriberKey(sub.IMSI)

	// 既存チェック
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return errors.New("subscriber already exists")
	}

	// created_atが未設定の場合は現在時刻を設定
	createdAt := sub.CreatedAt
	if createdAt == "" {
		createdAt = time.Now().UTC().Format(time.RFC3339)
	}

	return s.client.HSet(ctx, key, map[string]any{
		"ki":         sub.Ki,
		"opc":        sub.OPc,
		"amf":        sub.AMF,
		"sqn":        sub.SQN,
		"created_at": createdAt,
	}).Err()
}

// Update は既存の加入者を更新する。
func (s *SubscriberStore) Update(ctx context.Context, sub *model.Subscriber) error {
	key := SubscriberKey(sub.IMSI)

	// 存在チェック
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrSubscriberNotFound
	}

	return s.client.HSet(ctx, key, map[string]any{
		"ki":  sub.Ki,
		"opc": sub.OPc,
		"amf": sub.AMF,
		"sqn": sub.SQN,
	}).Err()
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
