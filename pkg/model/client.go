package model

// RadiusClient はRADIUSクライアント情報を表す。
// Valkeyキー: client:{IP}（Auth / Acct Server は送信元IPで引く）。ID からは idx:client:{ID} で IP を引く。
type RadiusClient struct {
	ID     int64  `json:"id"`     // サーバー採番のID（1 から始まる連番。再利用しない。0 は未採番）
	IP     string `json:"ip"`     // クライアントIPアドレス
	Secret string `json:"secret"` // 共有シークレット
	Name   string `json:"name"`   // クライアント名（識別用）
	Vendor string `json:"vendor"` // ベンダー名（任意）
}

// NewRadiusClient は新しいRadiusClientを生成する。
func NewRadiusClient(ip, secret, name, vendor string) *RadiusClient {
	return &RadiusClient{
		IP:     ip,
		Secret: secret,
		Name:   name,
		Vendor: vendor,
	}
}
