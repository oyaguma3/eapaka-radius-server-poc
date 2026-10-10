package model

import "encoding/json"

// 認可ポリシーの状態（policy:{IMSI} の status フィールド。D-02）
const (
	// PolicyStatusActive は利用中（status がないときもこれとみなす）
	PolicyStatusActive = "active"
	// PolicyStatusSuspended は停止中（Auth Server は認証を拒否する）
	PolicyStatusSuspended = "suspended"
)

// Policy は加入者のアクセスポリシーを表す。
// Valkeyキー: policy:{IMSI}
type Policy struct {
	IMSI      string       `json:"imsi"`       // 加入者IMSI
	Default   string       `json:"default"`    // デフォルトアクション（"allow" or "deny"）
	RulesJSON string       `json:"rules_json"` // ルールのJSON文字列（Valkey保存用）
	Rules     []PolicyRule `json:"-"`          // パース済みルール（メモリ上のみ）
	// Status は状態（"active" or "suspended"）。読み出しでは status がなければ "active"。
	// 作成・更新（Create / Update / Put）では書き込まない。変更は PolicyStore.SetStatus で行う
	Status string `json:"status"`
}

// PolicyRule はポリシールールを表す。
type PolicyRule struct {
	NasID          string   `json:"nas_id"`                    // NAS識別子（"*" 単独で任意のNASに一致。それ以外は完全一致）
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
		Status:    PolicyStatusActive,
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

// IsSuspended は停止中かどうかを返す。
func (p *Policy) IsSuspended() bool {
	return p.Status == PolicyStatusSuspended
}

// Clone はポリシーのディープコピーを作成する。
func (p *Policy) Clone() *Policy {
	clone := &Policy{
		IMSI:      p.IMSI,
		Default:   p.Default,
		RulesJSON: p.RulesJSON,
		Rules:     make([]PolicyRule, len(p.Rules)),
		Status:    p.Status,
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
