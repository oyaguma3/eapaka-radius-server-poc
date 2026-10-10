package policy

// 認可ポリシーの状態（policy:{IMSI} の status フィールド。D-02）
const (
	// StatusActive は利用中（status がないときもこれとみなす）
	StatusActive = "active"
	// StatusSuspended は停止中（認証を拒否する。D-09 セクション8.4.4）
	StatusSuspended = "suspended"
)

// Policy は認可ポリシーを表す（D-09 セクション8.3/8.4.2準拠）。
type Policy struct {
	Rules   []PolicyRule
	Default string // "allow" or "deny"
	Status  string // "active" or "suspended"
}

// IsSuspended は加入者が停止中かどうかを返す。
func (p *Policy) IsSuspended() bool {
	return p.Status == StatusSuspended
}

// PolicyRule は個別の認可ルールを表す（D-09 セクション8.3.3準拠）。
type PolicyRule struct {
	NasID          string   `json:"nas_id"`
	AllowedSSIDs   []string `json:"allowed_ssids"`
	VlanID         string   `json:"vlan_id,omitempty"`
	SessionTimeout int      `json:"session_timeout,omitempty"`
}

// EvaluationResult はポリシー評価結果を表す（D-09 セクション8.5.4準拠）。
type EvaluationResult struct {
	Allowed     bool
	MatchedRule *PolicyRule // nilの場合はdefault適用
	DenyReason  string      // Deny時の理由
}
