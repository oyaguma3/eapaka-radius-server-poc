package vector

import "context"

// VectorClient はVector Gatewayとの通信インターフェースを定義する
type VectorClient interface {
	// GetVector は認証ベクターを取得する
	GetVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error)
}
