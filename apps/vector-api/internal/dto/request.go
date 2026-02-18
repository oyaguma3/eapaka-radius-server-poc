// Package dto はリクエスト・レスポンスのデータ転送オブジェクトを定義する。
package dto

// VectorRequest はベクター生成リクエストを表す。
type VectorRequest struct {
	IMSI       string      `json:"imsi" binding:"required"`
	ResyncInfo *ResyncInfo `json:"resync_info,omitempty"`
}

// ResyncInfo は再同期情報を表す。
type ResyncInfo struct {
	RAND string `json:"rand" binding:"required"`
	AUTS string `json:"auts" binding:"required"`
}
