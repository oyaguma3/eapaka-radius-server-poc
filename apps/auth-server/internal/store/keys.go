package store

// Valkeyキープレフィックス（D-02準拠）
const (
	KeyPrefixSubscriber = "sub:"      // 加入者情報
	KeyPrefixClient     = "client:"   // RADIUSクライアント設定
	KeyPrefixPolicy     = "policy:"   // 認可ポリシー
	KeyPrefixEAPContext = "eap:"      // EAP認証コンテキスト
	KeyPrefixSession    = "sess:"     // アクティブセッション
	KeyPrefixUserIndex  = "idx:user:" // ユーザー検索インデックス
)
