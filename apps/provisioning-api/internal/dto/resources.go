package dto

import (
	"strings"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

// ---- 加入者（D-13 §3.1） ----

// SubscriberCreate は加入者の登録の要求。省略を区別するため、すべてポインターで受ける。
type SubscriberCreate struct {
	IMSI *string `json:"imsi"`
	Ki   *string `json:"ki"`
	OPc  *string `json:"opc"`
	AMF  *string `json:"amf"`
	SQN  *string `json:"sqn"`
}

// SubscriberUpdate は加入者の変更の要求（JSON Merge Patch）。
type SubscriberUpdate struct {
	Ki  Optional[string] `json:"ki"`
	OPc Optional[string] `json:"opc"`
	AMF Optional[string] `json:"amf"`
	SQN Optional[string] `json:"sqn"`
}

// IsEmpty は項目が1つも指定されていないかを返す。
func (u *SubscriberUpdate) IsEmpty() bool {
	return !u.Ki.Set && !u.OPc.Set && !u.AMF.Set && !u.SQN.Set
}

// Subscriber は加入者の応答。Ki と OPc は含めない。
type Subscriber struct {
	IMSI      string `json:"imsi"`
	AMF       string `json:"amf"`
	SQN       string `json:"sqn"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// NewSubscriber は応答用の加入者を作る。16進は小文字で返す（D-13 §3.5）。
func NewSubscriber(sub *model.Subscriber) Subscriber {
	return Subscriber{
		IMSI:      sub.IMSI,
		AMF:       strings.ToLower(sub.AMF),
		SQN:       strings.ToLower(sub.SQN),
		CreatedAt: formatTime(sub.CreatedAt),
	}
}

// SubscriberKeys は Ki と OPc の応答。
type SubscriberKeys struct {
	Ki  string `json:"ki"`
	OPc string `json:"opc"`
}

// NewSubscriberKeys は応答用の Ki と OPc を作る。
func NewSubscriberKeys(sub *model.Subscriber) SubscriberKeys {
	return SubscriberKeys{Ki: strings.ToLower(sub.Ki), OPc: strings.ToLower(sub.OPc)}
}

// SubscriberList は加入者の一覧の応答。
type SubscriberList struct {
	Items      []Subscriber `json:"items"`
	Total      int          `json:"total"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

// ---- RADIUSクライアント（D-13 §3.2） ----

// ClientCreate は RADIUSクライアントの登録の要求。
type ClientCreate struct {
	IP     *string `json:"ip"`
	Secret *string `json:"secret"`
	Name   *string `json:"name"`
	Vendor *string `json:"vendor"`
}

// ClientUpdate は RADIUSクライアントの変更の要求（JSON Merge Patch）。
type ClientUpdate struct {
	Secret Optional[string] `json:"secret"`
	Name   Optional[string] `json:"name"`
	Vendor Optional[string] `json:"vendor"`
}

// IsEmpty は項目が1つも指定されていないかを返す。
func (u *ClientUpdate) IsEmpty() bool {
	return !u.Secret.Set && !u.Name.Set && !u.Vendor.Set
}

// Client は RADIUSクライアントの応答。共有シークレットは含めない。
type Client struct {
	IP     string `json:"ip"`
	Name   string `json:"name"`
	Vendor string `json:"vendor"`
}

// NewClient は応答用の RADIUSクライアントを作る。
func NewClient(c *model.RadiusClient) Client {
	return Client{IP: c.IP, Name: c.Name, Vendor: c.Vendor}
}

// ClientSecret は共有シークレットの応答。
type ClientSecret struct {
	Secret string `json:"secret"`
}

// ClientList は RADIUSクライアントの一覧の応答。
type ClientList struct {
	Items []Client `json:"items"`
}

// ---- 認可ポリシー（D-13 §3.3） ----

// PolicyRuleInput は認可ポリシーのルールの要求。
// AllowedSSIDs は、省略・null（nil）と空の配列を区別する。
type PolicyRuleInput struct {
	NasID          *string  `json:"nasId"`
	AllowedSSIDs   []string `json:"allowedSsids"`
	VlanID         *string  `json:"vlanId"`
	SessionTimeout *int     `json:"sessionTimeout"`
}

// PolicyPut は認可ポリシーの作成・置き換えの要求。
// Rules は、省略・null（nil）と空の配列を区別する。
type PolicyPut struct {
	Default *string           `json:"default"`
	Rules   []PolicyRuleInput `json:"rules"`
}

// PolicyRule は認可ポリシーのルールの応答。
type PolicyRule struct {
	NasID          string   `json:"nasId"`
	AllowedSSIDs   []string `json:"allowedSsids"`
	VlanID         string   `json:"vlanId,omitempty"`
	SessionTimeout int      `json:"sessionTimeout,omitempty"`
}

// Policy は認可ポリシーの応答。
type Policy struct {
	IMSI    string       `json:"imsi"`
	Default string       `json:"default"`
	Rules   []PolicyRule `json:"rules"`
}

// NewPolicy は応答用の認可ポリシーを作る。Valkey の snake_case の JSON を camelCase に変換する。
func NewPolicy(p *model.Policy) Policy {
	rules := make([]PolicyRule, len(p.Rules))
	for i, r := range p.Rules {
		ssids := r.AllowedSSIDs
		if ssids == nil {
			ssids = []string{}
		}
		rules[i] = PolicyRule{NasID: r.NasID, AllowedSSIDs: ssids, VlanID: r.VlanID, SessionTimeout: r.SessionTimeout}
	}
	return Policy{IMSI: p.IMSI, Default: p.Default, Rules: rules}
}

// PolicyList は認可ポリシーの一覧の応答。
type PolicyList struct {
	Items      []Policy `json:"items"`
	Total      int      `json:"total"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// ---- 一覧の条件・状態 ----

// ListQuery は加入者・認可ポリシーの一覧のクエリパラメーター（検証前の値）。
type ListQuery struct {
	Prefix string
	Cursor string
	Limit  string
}

// Status は状態の応答（D-13 §4.4）。
type Status struct {
	Version         string    `json:"version"`
	NodeName        string    `json:"nodeName"`
	StartedAt       time.Time `json:"startedAt"`
	SubscriberCount int64     `json:"subscriberCount"`
	ClientCount     int64     `json:"clientCount"`
	PolicyCount     int64     `json:"policyCount"`
}

// formatTime は Valkey の日時（RFC 3339）を UTC の RFC 3339 にする。解釈できなければ空文字（応答では省略）を返す。
func formatTime(s string) string {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
