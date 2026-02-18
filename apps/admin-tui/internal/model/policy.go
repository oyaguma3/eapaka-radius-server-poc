// Package model はAdmin TUI専用のデータモデルを提供する。
package model

import "encoding/json"

// Policy は加入者のアクセスポリシーを表す（D-05/D-07準拠）。
// Valkeyキー: policy:{IMSI}
type Policy struct {
	IMSI      string       `json:"imsi"`       // 加入者IMSI
	Default   string       `json:"default"`    // デフォルトアクション（"allow" or "deny"）
	RulesJSON string       `json:"rules_json"` // ルールのJSON文字列（Valkey保存用）
	Rules     []PolicyRule `json:"-"`          // パース済みルール（メモリ上のみ）
}

// PolicyRule はポリシールールを表す（D-05/D-07準拠）。
type PolicyRule struct {
	NasID          string   `json:"nas_id"`                    // NAS識別子（ワイルドカード可）
	AllowedSSIDs   []string `json:"allowed_ssids"`             // 許可SSIDリスト
	VlanID         string   `json:"vlan_id,omitempty"`         // VLAN ID（空文字は未設定）
	SessionTimeout int      `json:"session_timeout,omitempty"` // セッションタイムアウト秒（0は未設定）
}

// NewPolicy は新しいPolicyを生成する。
func NewPolicy(imsi, defaultAction string) *Policy {
	return &Policy{
		IMSI:      imsi,
		Default:   defaultAction,
		RulesJSON: "[]",
		Rules:     []PolicyRule{},
	}
}

// ParseRules はRulesJSONをパースしてRulesに格納する。
func (p *Policy) ParseRules() error {
	if p.RulesJSON == "" || p.RulesJSON == "[]" {
		p.Rules = []PolicyRule{}
		return nil
	}
	return json.Unmarshal([]byte(p.RulesJSON), &p.Rules)
}

// EncodeRules はRulesをJSON文字列にエンコードしてRulesJSONに格納する。
func (p *Policy) EncodeRules() error {
	data, err := json.Marshal(p.Rules)
	if err != nil {
		return err
	}
	p.RulesJSON = string(data)
	return nil
}

// IsAllowByDefault はデフォルトアクションが許可かどうかを返す。
func (p *Policy) IsAllowByDefault() bool {
	return p.Default == "allow"
}

// Clone はポリシーのディープコピーを作成する。
func (p *Policy) Clone() *Policy {
	clone := &Policy{
		IMSI:      p.IMSI,
		Default:   p.Default,
		RulesJSON: p.RulesJSON,
		Rules:     make([]PolicyRule, len(p.Rules)),
	}
	for i, rule := range p.Rules {
		clone.Rules[i] = PolicyRule{
			NasID:          rule.NasID,
			AllowedSSIDs:   append([]string{}, rule.AllowedSSIDs...),
			VlanID:         rule.VlanID,
			SessionTimeout: rule.SessionTimeout,
		}
	}
	return clone
}
