// Package backend はバックエンドサービスへのアクセスを提供する。
package backend

import "context"

// VectorRequest はベクター生成リクエストを表す。
type VectorRequest struct {
	IMSI       string      `json:"imsi"`
	ResyncInfo *ResyncInfo `json:"resync_info,omitempty"`
}

// ResyncInfo は再同期情報を表す。
type ResyncInfo struct {
	RAND string `json:"rand"`
	AUTS string `json:"auts"`
}

// VectorResponse はベクター生成レスポンスを表す。
type VectorResponse struct {
	RAND string `json:"rand"`
	AUTN string `json:"autn"`
	XRES string `json:"xres"`
	CK   string `json:"ck"`
	IK   string `json:"ik"`
}

// Backend はベクター生成バックエンドのインターフェース。
type Backend interface {
	// GetVector はベクターを取得する。
	GetVector(ctx context.Context, req *VectorRequest) (*VectorResponse, error)
	// ID はバックエンドIDを返す。
	ID() string
	// Name はバックエンド名を返す。
	Name() string
}
