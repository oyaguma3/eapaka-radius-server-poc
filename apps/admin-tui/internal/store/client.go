package store

import (
	"context"
	"errors"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/redis/go-redis/v9"
)

// ErrClientNotFound はRADIUSクライアントが見つからない場合のエラー
var ErrClientNotFound = errors.New("client not found")

// ClientStore はRADIUSクライアントデータへのアクセスを提供する。
type ClientStore struct {
	client *redis.Client
}

// NewClientStore は新しいClientStoreを生成する。
func NewClientStore(client *redis.Client) *ClientStore {
	return &ClientStore{client: client}
}

// Get は指定されたIPのRADIUSクライアントを取得する。
// Auth Server/Acct Serverと互換性のあるHash形式で読み取る。
func (s *ClientStore) Get(ctx context.Context, ip string) (*model.RadiusClient, error) {
	key := ClientKey(ip)
	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// キーが存在しない場合、HGetAllは空mapを返す
	if len(result) == 0 {
		return nil, ErrClientNotFound
	}

	return clientFromHash(ip, result), nil
}

// Create は新しいRADIUSクライアントを作成する。
// Auth Server/Acct Serverと互換性のあるHash形式で保存する。
func (s *ClientStore) Create(ctx context.Context, c *model.RadiusClient) error {
	key := ClientKey(c.IP)

	// 既存チェック
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return errors.New("client already exists")
	}

	return s.client.HSet(ctx, key, map[string]any{
		"secret": c.Secret,
		"name":   c.Name,
		"vendor": c.Vendor,
	}).Err()
}

// Update は既存のRADIUSクライアントを更新する。
func (s *ClientStore) Update(ctx context.Context, c *model.RadiusClient) error {
	key := ClientKey(c.IP)

	// 存在チェック
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrClientNotFound
	}

	return s.client.HSet(ctx, key, map[string]any{
		"secret": c.Secret,
		"name":   c.Name,
		"vendor": c.Vendor,
	}).Err()
}

// Delete はRADIUSクライアントを削除する。
func (s *ClientStore) Delete(ctx context.Context, ip string) error {
	key := ClientKey(ip)

	result, err := s.client.Del(ctx, key).Result()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrClientNotFound
	}
	return nil
}

// List は全RADIUSクライアントのリストを取得する（SCAN使用）。
func (s *ClientStore) List(ctx context.Context) ([]*model.RadiusClient, error) {
	var clients []*model.RadiusClient
	var keys []string

	// SCANで全キーを取得
	iter := s.client.Scan(ctx, 0, PrefixClient+"*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return clients, nil
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

		// キーからIPを抽出
		ip := keys[i][len(PrefixClient):]
		clients = append(clients, clientFromHash(ip, result))
	}

	return clients, nil
}

// Count はRADIUSクライアントの総数を返す。
func (s *ClientStore) Count(ctx context.Context) (int64, error) {
	var count int64

	iter := s.client.Scan(ctx, 0, PrefixClient+"*", 100).Iterator()
	for iter.Next(ctx) {
		count++
	}
	if err := iter.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

// Exists は指定されたIPのRADIUSクライアントが存在するか確認する。
func (s *ClientStore) Exists(ctx context.Context, ip string) (bool, error) {
	key := ClientKey(ip)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// BulkCreate は複数のRADIUSクライアントを一括で作成する（TxPipeline使用）。
func (s *ClientStore) BulkCreate(ctx context.Context, clients []*model.RadiusClient) error {
	if len(clients) == 0 {
		return nil
	}

	pipe := s.client.TxPipeline()
	for _, c := range clients {
		key := ClientKey(c.IP)
		pipe.HSet(ctx, key, map[string]any{
			"secret": c.Secret,
			"name":   c.Name,
			"vendor": c.Vendor,
		})
	}

	_, err := pipe.Exec(ctx)
	return err
}

// clientFromHash はHashマップからRadiusClientを構築する。
func clientFromHash(ip string, fields map[string]string) *model.RadiusClient {
	return &model.RadiusClient{
		IP:     ip,
		Secret: fields["secret"],
		Name:   fields["name"],
		Vendor: fields["vendor"],
	}
}
