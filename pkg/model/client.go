package model

// RadiusClient はRADIUSクライアント情報を表す。
// Valkeyキー: client:{IP}
type RadiusClient struct {
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
