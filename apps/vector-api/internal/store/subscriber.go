package store

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Subscriber は加入者情報を表す。
type Subscriber struct {
	IMSI string
	Ki   string // Hex 32桁
	OPc  string // Hex 32桁
	AMF  string // Hex 4桁
	SQN  string // Hex 12桁
}

// SubscriberStore は加入者データへのアクセスを提供する。
type SubscriberStore struct {
	client *ValkeyClient
}

// NewSubscriberStore は新しいSubscriberStoreを生成する。
func NewSubscriberStore(client *ValkeyClient) *SubscriberStore {
	return &SubscriberStore{client: client}
}

// Get は加入者情報を取得する。
// キー: sub:{IMSI}
func (s *SubscriberStore) Get(ctx context.Context, imsi string) (*Subscriber, error) {
	key := "sub:" + imsi

	result, err := s.client.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get subscriber: %w", err)
	}

	if len(result) == 0 {
		return nil, nil // 未登録
	}

	return &Subscriber{
		IMSI: imsi,
		Ki:   result["ki"],
		OPc:  result["opc"],
		AMF:  result["amf"],
		SQN:  result["sqn"],
	}, nil
}

// compareAndSetSQNScript は sqn が期待値と一致するときだけ新しい値に書き換える。
// キーまたは sqn フィールドが無い場合は書き換えず（キーを新たに作らない）、0 を返す。
var compareAndSetSQNScript = redis.NewScript(`
local cur = redis.call('HGET', KEYS[1], 'sqn')
if cur == false or cur ~= ARGV[1] then
  return 0
end
redis.call('HSET', KEYS[1], 'sqn', ARGV[2])
return 1
`)

// CompareAndSetSQN は加入者のSQNが oldSQN と一致するときだけ newSQN に更新する。
// 更新したときは true を返す。一致しない（他のリクエストが先に更新した）ときや、
// 加入者が削除されていたときは false を返す。
// oldSQN には Get で読んだ値をそのまま渡す（大文字小文字を含めて文字列で比較する）。
func (s *SubscriberStore) CompareAndSetSQN(ctx context.Context, imsi, oldSQN, newSQN string) (bool, error) {
	key := "sub:" + imsi

	n, err := compareAndSetSQNScript.Run(ctx, s.client.client, []string{key}, oldSQN, newSQN).Int()
	if err != nil {
		return false, fmt.Errorf("failed to compare and set SQN: %w", err)
	}

	return n == 1, nil
}
