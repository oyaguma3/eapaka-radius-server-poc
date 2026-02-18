package vector

// VectorRequest はVector Gatewayへのリクエストを表す
type VectorRequest struct {
	IMSI       string      `json:"imsi"`
	ResyncInfo *ResyncInfo `json:"resync_info,omitempty"`
}

// ResyncInfo は再同期情報を表す
type ResyncInfo struct {
	RAND string `json:"rand"` // Hex文字列
	AUTS string `json:"auts"` // Hex文字列
}

// VectorResponse はVector Gatewayからのレスポンスを表す
type VectorResponse struct {
	RAND []byte // 16バイト
	AUTN []byte // 16バイト
	XRES []byte // 4-16バイト
	CK   []byte // 16バイト
	IK   []byte // 16バイト
}

// vectorResponseJSON はJSONパース用の内部構造体
type vectorResponseJSON struct {
	RAND string `json:"rand"`
	AUTN string `json:"autn"`
	XRES string `json:"xres"`
	CK   string `json:"ck"`
	IK   string `json:"ik"`
}

// ProblemDetails はRFC 7807エラーレスポンスを表す
type ProblemDetails struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}
