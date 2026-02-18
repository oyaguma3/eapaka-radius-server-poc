package model

import "encoding/json"

// Policy は加入者のアクセスポリシーを表す。
// Valkeyキー: policy:{IMSI}
type Policy struct {
	IMSI      string       `json:"imsi"`       // 加入者IMSI
	Default   string       `json:"default"`    // デフォルトアクション（"allow" or "deny"）
	RulesJSON string       `json:"rules_json"` // ルールのJSON文字列（Valkey保存用）
	Rules     []PolicyRule `json:"-"`          // パース済みルール（メモリ上のみ）
}

// PolicyRule はポリシールールを表す。
type PolicyRule struct {
	SSID    string `json:"ssid"`     // 対象SSID（ワイルドカード可）
	Action  string `json:"action"`   // アクション（"allow" or "deny"）
	TimeMin string `json:"time_min"` // 許可開始時刻（HH:MM形式、空で制限なし）
	TimeMax string `json:"time_max"` // 許可終了時刻（HH:MM形式、空で制限なし）
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
