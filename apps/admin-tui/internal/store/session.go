package store

import (
	"context"
	"encoding/json"
	"errors"
	"log"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// ErrSessionNotFound はセッションが見つからない場合のエラー
var ErrSessionNotFound = errors.New("session not found")

// SessionStore はセッションデータへのアクセスを提供する。
type SessionStore struct {
	client *redis.Client
}

// NewSessionStore は新しいSessionStoreを生成する。
func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{client: client}
}

// Get は指定されたUUIDのセッションを取得する。
func (s *SessionStore) Get(ctx context.Context, uuid string) (*model.Session, error) {
	key := SessionKey(uuid)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	var session model.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// List は全セッションのリストを取得する（SCAN使用）。
func (s *SessionStore) List(ctx context.Context) ([]*model.Session, error) {
	var sessions []*model.Session
	var keys []string

	// SCANで全キーを取得
	iter := s.client.Scan(ctx, 0, PrefixSession+"*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return sessions, nil
	}

	// Pipelineで一括取得
	pipe := s.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.Get(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	for _, cmd := range cmds {
		data, err := cmd.Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				continue
			}
			return nil, err
		}

		var session model.Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		sessions = append(sessions, &session)
	}

	return sessions, nil
}

// Count はセッションの総数を返す。
func (s *SessionStore) Count(ctx context.Context) (int64, error) {
	var count int64

	iter := s.client.Scan(ctx, 0, PrefixSession+"*", 100).Iterator()
	for iter.Next(ctx) {
		count++
	}
	if err := iter.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

// GetByIMSI は指定されたIMSIのセッションリストを取得する（idx:user経由）。
// 存在しないセッションはインデックスからクリーンアップする。
func (s *SessionStore) GetByIMSI(ctx context.Context, imsi string) ([]*model.Session, error) {
	indexKey := UserIndexKey(imsi)

	// idx:user:{IMSI} から全セッションUUIDを取得
	uuids, err := s.client.SMembers(ctx, indexKey).Result()
	if err != nil {
		return nil, err
	}

	if len(uuids) == 0 {
		return []*model.Session{}, nil
	}

	var sessions []*model.Session
	var staleUUIDs []string

	// Pipelineで一括取得
	pipe := s.client.Pipeline()
	cmds := make([]*redis.StringCmd, len(uuids))
	for i, uuid := range uuids {
		cmds[i] = pipe.Get(ctx, SessionKey(uuid))
	}
	_, err = pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	for i, cmd := range cmds {
		data, err := cmd.Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				// セッションが存在しない場合はstaleとしてマーク
				staleUUIDs = append(staleUUIDs, uuids[i])
				continue
			}
			return nil, err
		}

		var session model.Session
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		sessions = append(sessions, &session)
	}

	// 存在しないセッションをインデックスからクリーンアップ（失敗はログのみ）
	if len(staleUUIDs) > 0 {
		for _, uuid := range staleUUIDs {
			if err := s.client.SRem(ctx, indexKey, uuid).Err(); err != nil {
				log.Printf("failed to cleanup stale session from index: imsi=%s, uuid=%s, err=%v", imsi, uuid, err)
			}
		}
	}

	return sessions, nil
}

// GetSessionCount は指定されたIMSIのセッション数を返す。
func (s *SessionStore) GetSessionCount(ctx context.Context, imsi string) (int64, error) {
	indexKey := UserIndexKey(imsi)
	return s.client.SCard(ctx, indexKey).Result()
}
