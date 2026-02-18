package session

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/store"
)

// Session はアクティブセッションを表す（D-09 セクション9.3.1準拠）。
type Session struct {
	IMSI         string `redis:"imsi"`
	NasIP        string `redis:"nas_ip"`
	StartTime    int64  `redis:"start_time"`
	ClientIP     string `redis:"client_ip"`
	AcctID       string `redis:"acct_id"`
	InputOctets  int64  `redis:"input_octets"`
	OutputOctets int64  `redis:"output_octets"`
}

// sessionStore はSessionStoreの実装。
type sessionStore struct {
	vc *store.ValkeyClient
}

// NewSessionStore はSessionStoreの新しいインスタンスを生成する。
func NewSessionStore(vc *store.ValkeyClient) SessionStore {
	return &sessionStore{vc: vc}
}

// Create はセッションをValkeyに保存する。
func (s *sessionStore) Create(ctx context.Context, sessionID string, sess *Session) error {
	key := store.KeyPrefixSession + sessionID
	m := store.StructToMap(sess)

	pipe := s.vc.Client().Pipeline()
	pipe.HSet(ctx, key, m)
	pipe.Expire(ctx, key, config.SessionTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("%w: %v", store.ErrValkeyUnavailable, err)
	}
	return nil
}

// Get はセッションをValkeyから取得する。
func (s *sessionStore) Get(ctx context.Context, sessionID string) (*Session, error) {
	key := store.KeyPrefixSession + sessionID
	m, err := s.vc.Client().HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", store.ErrValkeyUnavailable, err)
	}
	if len(m) == 0 {
		return nil, ErrSessionNotFound
	}

	var sess Session
	if err := store.MapToStruct(m, &sess); err != nil {
		return nil, fmt.Errorf("session deserialization error: %w", err)
	}
	return &sess, nil
}

// AddUserIndex はIMSIとセッションIDの紐付けをSet型で追加する。
func (s *sessionStore) AddUserIndex(ctx context.Context, imsi string, sessionID string) error {
	key := store.KeyPrefixUserIndex + imsi
	if err := s.vc.Client().SAdd(ctx, key, sessionID).Err(); err != nil {
		return fmt.Errorf("%w: %v", store.ErrValkeyUnavailable, err)
	}
	return nil
}

// GenerateSessionID はUUID形式のセッションIDを生成する。
func GenerateSessionID() string {
	return uuid.New().String()
}
