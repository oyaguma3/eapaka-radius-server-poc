package masterdata

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// セッション（sess:{UUID}）と、IMSI からセッションを引く索引（idx:user:{IMSI}）のキー（D-02 §3.E・§3.F）。
// マスタデータではないが、Admin TUI と Provisioning API が同じ読み方をするため、読み出しだけをここに置く
// （書き込みは Auth / Acct Server が行う）。
const (
	// PrefixSession はセッションキーのプレフィックス
	PrefixSession = "sess:"
	// PrefixUserIndex は IMSI からセッションを引く索引のプレフィックス
	PrefixUserIndex = "idx:user:"
)

// SessionKey はセッションのValkeyキーを生成する。
func SessionKey(uuid string) string {
	return PrefixSession + uuid
}

// UserIndexKey は IMSI からセッションを引く索引のキーを生成する。
func UserIndexKey(imsi string) string {
	return PrefixUserIndex + imsi
}

// ErrSessionNotFound はセッションが見つからない場合のエラー
var ErrSessionNotFound = errors.New("session not found")

// SessionStore はセッションを読み出す。書き込み（索引の掃除を含む）はしない。
type SessionStore struct {
	client *redis.Client
}

// NewSessionStore は新しい SessionStore を生成する。
func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{client: client}
}

// Get は指定されたUUIDのセッションを取得する。
func (s *SessionStore) Get(ctx context.Context, uuid string) (*model.Session, error) {
	m, err := s.client.HGetAll(ctx, SessionKey(uuid)).Result()
	if err != nil {
		return nil, err
	}
	if len(m) == 0 {
		return nil, ErrSessionNotFound
	}
	return sessionFromHash(uuid, m)
}

// List は全セッションを取得する（SCAN）。値を解釈できないセッションは除く。
func (s *SessionStore) List(ctx context.Context) ([]*model.Session, error) {
	var uuids []string
	iter := s.client.Scan(ctx, 0, PrefixSession+"*", 100).Iterator()
	for iter.Next(ctx) {
		uuids = append(uuids, strings.TrimPrefix(iter.Val(), PrefixSession))
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	sessions, _, err := s.getMany(ctx, uuids)
	return sessions, err
}

// Count はセッションの総数を返す（SCAN）。
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

// ListByIMSI は指定された IMSI のセッションを取得する（idx:user:{IMSI} 経由）。
// 索引にあるがセッションがない UUID（TTL 切れや Acct-Stop の未着で残ったもの）を stale として返す（索引からは消さない）。
// 索引が空の場合は、全セッションを SCAN して IMSI で絞り込む（D-02 §3.F）。
func (s *SessionStore) ListByIMSI(ctx context.Context, imsi string) (sessions []*model.Session, stale []string, err error) {
	uuids, err := s.client.SMembers(ctx, UserIndexKey(imsi)).Result()
	if err != nil {
		return nil, nil, err
	}
	if len(uuids) == 0 {
		all, err := s.List(ctx)
		if err != nil {
			return nil, nil, err
		}
		sessions = []*model.Session{}
		for _, sess := range all {
			if sess.IMSI == imsi {
				sessions = append(sessions, sess)
			}
		}
		return sessions, nil, nil
	}
	return s.getMany(ctx, uuids)
}

// IndexCount は指定された IMSI の索引にあるセッションの数を返す（SCARD。掃除前の古い UUID を含みうる）。
func (s *SessionStore) IndexCount(ctx context.Context, imsi string) (int64, error) {
	return s.client.SCard(ctx, UserIndexKey(imsi)).Result()
}

// getMany は UUID のセッションをまとめて取得する。存在しない UUID は missing として返し、
// 値を解釈できないセッションは除く。
func (s *SessionStore) getMany(ctx context.Context, uuids []string) (sessions []*model.Session, missing []string, err error) {
	sessions = []*model.Session{}
	if len(uuids) == 0 {
		return sessions, nil, nil
	}
	pipe := s.client.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(uuids))
	for i, uuid := range uuids {
		cmds[i] = pipe.HGetAll(ctx, SessionKey(uuid))
	}
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		return nil, nil, err
	}
	for i, cmd := range cmds {
		m, err := cmd.Result()
		if err != nil {
			continue
		}
		if len(m) == 0 {
			missing = append(missing, uuids[i])
			continue
		}
		sess, err := sessionFromHash(uuids[i], m)
		if err != nil {
			continue
		}
		sessions = append(sessions, sess)
	}
	return sessions, missing, nil
}

// sessionFromHash は sess:{UUID} の Hash を model.Session にする（フィールド名は Auth / Acct Server と同じ。D-02 §3.E）。
func sessionFromHash(uuid string, m map[string]string) (*model.Session, error) {
	sess := &model.Session{
		UUID:          uuid,
		IMSI:          m["imsi"],
		NasIP:         m["nas_ip"],
		NasIdentifier: m["nas_identifier"],
		ClientIP:      m["client_ip"],
		AcctSessionID: m["acct_id"],
	}
	for _, f := range []struct {
		name string
		dst  *int64
	}{
		{"start_time", &sess.StartTime},
		{"input_octets", &sess.InputOctets},
		{"output_octets", &sess.OutputOctets},
	} {
		v := m[f.name]
		if v == "" {
			continue
		}
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", f.name, err)
		}
		*f.dst = n
	}
	return sess, nil
}
