package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"

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
	m, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, ErrSessionNotFound
	}

	session, err := mapToSession(uuid, m)
	if err != nil {
		return nil, err
	}
	return session, nil
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
	cmds := make([]*redis.MapStringStringCmd, len(keys))
	for i, key := range keys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	for i, cmd := range cmds {
		m, err := cmd.Result()
		if err != nil {
			continue
		}
		if len(m) == 0 {
			continue
		}

		uuid := strings.TrimPrefix(keys[i], PrefixSession)
		session, err := mapToSession(uuid, m)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
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

	// idx:user インデックスが空の場合は SCAN フォールバック
	if len(uuids) == 0 {
		return s.getByIMSIScan(ctx, imsi)
	}

	var sessions []*model.Session
	var staleUUIDs []string

	// Pipelineで一括取得
	pipe := s.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(uuids))
	for i, uuid := range uuids {
		cmds[i] = pipe.HGetAll(ctx, SessionKey(uuid))
	}
	_, err = pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	for i, cmd := range cmds {
		m, err := cmd.Result()
		if err != nil {
			continue
		}
		if len(m) == 0 {
			// セッションが存在しない場合はstaleとしてマーク
			staleUUIDs = append(staleUUIDs, uuids[i])
			continue
		}

		session, err := mapToSession(uuids[i], m)
		if err != nil {
			continue
		}
		sessions = append(sessions, session)
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

// getByIMSIScan は全セッションを SCAN して IMSI でフィルタリングする（フォールバック用）。
func (s *SessionStore) getByIMSIScan(ctx context.Context, imsi string) ([]*model.Session, error) {
	allSessions, err := s.List(ctx)
	if err != nil {
		return nil, err
	}
	var sessions []*model.Session
	for _, sess := range allSessions {
		if sess.IMSI == imsi {
			sessions = append(sessions, sess)
		}
	}
	if sessions == nil {
		sessions = []*model.Session{}
	}
	return sessions, nil
}

// GetSessionCount は指定されたIMSIのセッション数を返す。
func (s *SessionStore) GetSessionCount(ctx context.Context, imsi string) (int64, error) {
	indexKey := UserIndexKey(imsi)
	return s.client.SCard(ctx, indexKey).Result()
}

// mapToSession はRedis Hashのmap[string]stringからmodel.Sessionに変換する。
// Auth/Acctサーバーのredisタグに合わせたフィールド名でマッピングする。
func mapToSession(uuid string, m map[string]string) (*model.Session, error) {
	session := &model.Session{
		UUID:          uuid,
		IMSI:          m["imsi"],
		NasIP:         m["nas_ip"],
		ClientIP:      m["client_ip"],
		AcctSessionID: m["acct_id"],
	}

	if v, ok := m["start_time"]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid start_time: %w", err)
		}
		session.StartTime = n
	}

	if v, ok := m["input_octets"]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid input_octets: %w", err)
		}
		session.InputOctets = n
	}

	if v, ok := m["output_octets"]; ok && v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid output_octets: %w", err)
		}
		session.OutputOctets = n
	}

	return session, nil
}
