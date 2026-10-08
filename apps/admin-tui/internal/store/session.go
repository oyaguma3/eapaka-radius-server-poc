package store

import (
	"context"
	"log"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// ErrSessionNotFound はセッションが見つからない場合のエラー
var ErrSessionNotFound = masterdata.ErrSessionNotFound

// SessionStore はセッションデータへのアクセスを提供する。
// 読み出しは Provisioning API と共通の pkg/masterdata.SessionStore で行い、
// IMSI で読むときの索引の掃除（存在しないセッションの UUID を idx:user:{IMSI} から消す）だけをここで行う。
type SessionStore struct {
	client *redis.Client
	read   *masterdata.SessionStore
}

// NewSessionStore は新しいSessionStoreを生成する。
func NewSessionStore(client *redis.Client) *SessionStore {
	return &SessionStore{client: client, read: masterdata.NewSessionStore(client)}
}

// Get は指定されたUUIDのセッションを取得する。
func (s *SessionStore) Get(ctx context.Context, uuid string) (*model.Session, error) {
	return s.read.Get(ctx, uuid)
}

// List は全セッションのリストを取得する（SCAN使用）。
func (s *SessionStore) List(ctx context.Context) ([]*model.Session, error) {
	return s.read.List(ctx)
}

// Count はセッションの総数を返す。
func (s *SessionStore) Count(ctx context.Context) (int64, error) {
	return s.read.Count(ctx)
}

// GetByIMSI は指定されたIMSIのセッションリストを取得する（idx:user経由）。
// 存在しないセッションはインデックスからクリーンアップする。
// インデックスが空の場合は全セッションを SCAN して IMSI で絞り込む。
func (s *SessionStore) GetByIMSI(ctx context.Context, imsi string) ([]*model.Session, error) {
	sessions, stale, err := s.read.ListByIMSI(ctx, imsi)
	if err != nil {
		return nil, err
	}
	// 存在しないセッションをインデックスからクリーンアップ（失敗はログのみ）
	indexKey := UserIndexKey(imsi)
	for _, uuid := range stale {
		if err := s.client.SRem(ctx, indexKey, uuid).Err(); err != nil {
			log.Printf("failed to cleanup stale session from index: imsi=%s, uuid=%s, err=%v", imsi, uuid, err)
		}
	}
	return sessions, nil
}

// GetSessionCount は指定されたIMSIのセッション数を返す。
func (s *SessionStore) GetSessionCount(ctx context.Context, imsi string) (int64, error) {
	return s.read.IndexCount(ctx, imsi)
}
