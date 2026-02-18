package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/policy"
)

// policyStore はpolicy.PolicyStoreインターフェースの実装。
type policyStore struct {
	vc *ValkeyClient
}

// NewPolicyStore は新しいPolicyStoreを生成する。
func NewPolicyStore(vc *ValkeyClient) policy.PolicyStore {
	return &policyStore{vc: vc}
}

// GetPolicy は指定されたIMSIに対応するポリシーを取得する。
func (s *policyStore) GetPolicy(ctx context.Context, imsi string) (*policy.Policy, error) {
	key := KeyPrefixPolicy + imsi
	result, err := s.vc.Client().HGetAll(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValkeyUnavailable, err)
	}

	// キーが存在しない場合、HGetAllは空mapを返す
	if len(result) == 0 {
		return nil, policy.ErrPolicyNotFound
	}

	p := &policy.Policy{}

	// defaultフィールドの取得
	defaultVal, ok := result["default"]
	if !ok || (defaultVal != "allow" && defaultVal != "deny") {
		p.Default = "deny"
	} else {
		p.Default = defaultVal
	}

	// rulesフィールドのJSONデシリアライズ
	rulesJSON, ok := result["rules"]
	if ok && rulesJSON != "" {
		var rules []policy.PolicyRule
		if err := json.Unmarshal([]byte(rulesJSON), &rules); err != nil {
			return nil, fmt.Errorf("%w: rules JSON parse error: %v", policy.ErrPolicyInvalid, err)
		}
		p.Rules = rules
	} else {
		p.Rules = []policy.PolicyRule{}
	}

	return p, nil
}
