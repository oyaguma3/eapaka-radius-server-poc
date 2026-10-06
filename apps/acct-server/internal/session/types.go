package session

// Session はアクティブセッションを表す（D-10 セクション9.1準拠）。
type Session struct {
	IMSI          string `redis:"imsi"`
	StartTime     int64  `redis:"start_time"`
	NasIP         string `redis:"nas_ip"`
	NasIdentifier string `redis:"nas_identifier"` // NAS-Identifier（プロキシ経由でもNASを識別できる）
	ClientIP      string `redis:"client_ip"`
	AcctID        string `redis:"acct_id"`
	InputOctets   int64  `redis:"input_octets"`
	OutputOctets  int64  `redis:"output_octets"`
}

// SessionStartData はAcct-Start処理で更新するフィールド
type SessionStartData struct {
	StartTime     int64
	NasIP         string
	NasIdentifier string // 空ならセッションの値（Auth Serverが書いた値）を残す
	AcctID        string
	ClientIP      string
}

// SessionInterimData はAcct-Interim処理で更新するフィールド
type SessionInterimData struct {
	NasIP         string
	NasIdentifier string // 空ならセッションの値を残す
	ClientIP      string
	InputOctets   int64
	OutputOctets  int64
}
