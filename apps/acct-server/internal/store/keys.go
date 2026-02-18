package store

// Valkeyキープレフィックス（D-02/D-10準拠）
const (
	KeyPrefixSession   = "sess:"      // アクティブセッション
	KeyPrefixUserIndex = "idx:user:"  // ユーザー検索インデックス
	KeyPrefixAcctSeen  = "acct:seen:" // 重複検出用
	KeyPrefixClient    = "client:"    // RADIUSクライアント設定
)
