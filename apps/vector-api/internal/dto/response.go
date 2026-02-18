package dto

// VectorResponse はベクター生成レスポンスを表す。
type VectorResponse struct {
	RAND string `json:"rand"`
	AUTN string `json:"autn"`
	XRES string `json:"xres"`
	CK   string `json:"ck"`
	IK   string `json:"ik"`
}

// HealthResponse はヘルスチェックレスポンスを表す。
type HealthResponse struct {
	Status string `json:"status"`
}
