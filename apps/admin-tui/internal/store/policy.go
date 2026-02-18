package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/model"
	"github.com/redis/go-redis/v9"
)

// ErrPolicyNotFound はポリシーが見つからない場合のエラー
var ErrPolicyNotFound = errors.New("policy not found")

// PolicyStore は認可ポリシーデータへのアクセスを提供する。
type PolicyStore struct {
	client *redis.Client
}

// NewPolicyStore は新しいPolicyStoreを生成する。
func NewPolicyStore(client *redis.Client) *PolicyStore {
	return &PolicyStore{client: client}
}

// Get は指定されたIMSIのポリシーを取得する。
// Auth Serverと互換性のあるHash形式で読み取る。
func (s *PolicyStore) Get(ctx context.Context, imsi string) (*model.Policy, error) {
	key := PolicyKey(imsi)
	result, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	// キーが存在しない場合、HGetAllは空mapを返す
	if len(result) == 0 {
		return nil, ErrPolicyNotFound
	}

	policy := &model.Policy{
		IMSI: imsi,
	}

	// defaultフィールドの取得
	if defaultVal, ok := result["default"]; ok {
		policy.Default = defaultVal
	} else {
		policy.Default = "deny" // デフォルト値
	}

	// rulesフィールドのJSONデシリアライズ
	if rulesJSON, ok := result["rules"]; ok && rulesJSON != "" {
		policy.RulesJSON = rulesJSON
		if err := json.Unmarshal([]byte(rulesJSON), &policy.Rules); err != nil {
			return nil, err
		}
	} else {
		policy.RulesJSON = "[]"
		policy.Rules = []model.PolicyRule{}
	}

	return policy, nil
}

// Create は新しいポリシーを作成する。
// Auth Serverと互換性のあるHash形式で保存する。
func (s *PolicyStore) Create(ctx context.Context, policy *model.Policy) error {
	key := PolicyKey(policy.IMSI)

	// 既存チェック
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return errors.New("policy already exists")
	}

	return s.saveAsHash(ctx, key, policy)
}

// Update は既存のポリシーを更新する。
func (s *PolicyStore) Update(ctx context.Context, policy *model.Policy) error {
	key := PolicyKey(policy.IMSI)

	// 存在チェック
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return ErrPolicyNotFound
	}

	return s.saveAsHash(ctx, key, policy)
}

// Upsert はポリシーを作成または更新する。
func (s *PolicyStore) Upsert(ctx context.Context, policy *model.Policy) error {
	key := PolicyKey(policy.IMSI)
	return s.saveAsHash(ctx, key, policy)
}

// saveAsHash はポリシーをHash形式で保存する内部メソッド。
func (s *PolicyStore) saveAsHash(ctx context.Context, key string, policy *model.Policy) error {
	// RulesをJSONにエンコード
	rulesJSON := "[]"
	if len(policy.Rules) > 0 {
		data, err := json.Marshal(policy.Rules)
		if err != nil {
			return err
		}
		rulesJSON = string(data)
	} else if policy.RulesJSON != "" {
		rulesJSON = policy.RulesJSON
	}

	// Hash形式で保存
	return s.client.HSet(ctx, key, map[string]interface{}{
		"default": policy.Default,
		"rules":   rulesJSON,
	}).Err()
}

// Delete はポリシーを削除する。
func (s *PolicyStore) Delete(ctx context.Context, imsi string) error {
	key := PolicyKey(imsi)

	result, err := s.client.Del(ctx, key).Result()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrPolicyNotFound
	}
	return nil
}

// List は全ポリシーのリストを取得する（SCAN使用）。
func (s *PolicyStore) List(ctx context.Context) ([]*model.Policy, error) {
	var policies []*model.Policy
	var keys []string

	// SCANで全キーを取得
	iter := s.client.Scan(ctx, 0, PrefixPolicy+"*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return policies, nil
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
		imsi := keys[i][len(PrefixPolicy):]

		policy := &model.Policy{
			IMSI: imsi,
		}

		// defaultフィールドの取得
		if defaultVal, ok := result["default"]; ok {
			policy.Default = defaultVal
		} else {
			policy.Default = "deny"
		}

		// rulesフィールドのJSONデシリアライズ
		if rulesJSON, ok := result["rules"]; ok && rulesJSON != "" {
			policy.RulesJSON = rulesJSON
			if err := json.Unmarshal([]byte(rulesJSON), &policy.Rules); err != nil {
				continue
			}
		} else {
			policy.RulesJSON = "[]"
			policy.Rules = []model.PolicyRule{}
		}

		policies = append(policies, policy)
	}

	return policies, nil
}

// Exists は指定されたIMSIのポリシーが存在するか確認する。
func (s *PolicyStore) Exists(ctx context.Context, imsi string) (bool, error) {
	key := PolicyKey(imsi)
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// BulkCreate は複数のポリシーを一括で作成する（TxPipeline使用）。
func (s *PolicyStore) BulkCreate(ctx context.Context, policies []*model.Policy) error {
	if len(policies) == 0 {
		return nil
	}

	pipe := s.client.TxPipeline()
	for _, policy := range policies {
		key := PolicyKey(policy.IMSI)

		// RulesをJSONにエンコード
		rulesJSON := "[]"
		if len(policy.Rules) > 0 {
			data, err := json.Marshal(policy.Rules)
			if err != nil {
				return err
			}
			rulesJSON = string(data)
		} else if policy.RulesJSON != "" {
			rulesJSON = policy.RulesJSON
		}

		// Hash形式で保存
		pipe.HSet(ctx, key, map[string]interface{}{
			"default": policy.Default,
			"rules":   rulesJSON,
		})
	}

	_, err := pipe.Exec(ctx)
	return err
}

// GetIMSIsWithPolicy はポリシーが設定されているIMSIのセットを返す。
func (s *PolicyStore) GetIMSIsWithPolicy(ctx context.Context) (map[string]bool, error) {
	result := make(map[string]bool)

	iter := s.client.Scan(ctx, 0, PrefixPolicy+"*", 100).Iterator()
	for iter.Next(ctx) {
		key := iter.Val()
		// "policy:" プレフィックスを除去してIMSIを取得
		if len(key) > len(PrefixPolicy) {
			imsi := key[len(PrefixPolicy):]
			result[imsi] = true
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	return result, nil
}
