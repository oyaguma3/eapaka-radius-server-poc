package model

// Stage はEAP認証のステージを表す定数。
type Stage string

const (
	// StageNew は新規セッション
	StageNew Stage = "new"
	// StageWaitingIdentity はIdentity待ち状態
	StageWaitingIdentity Stage = "waiting_identity"
	// StageIdentityReceived はIdentity受信済み状態
	StageIdentityReceived Stage = "identity_received"
	// StageWaitingVector はVector待ち状態
	StageWaitingVector Stage = "waiting_vector"
	// StageChallengeSent はChallenge送信済み状態
	StageChallengeSent Stage = "challenge_sent"
	// StageResyncSent は再同期要求送信済み状態
	StageResyncSent Stage = "resync_sent"
	// StageSuccess は認証成功状態
	StageSuccess Stage = "success"
	// StageFailure は認証失敗状態
	StageFailure Stage = "failure"
)

// Session はRADIUSセッション情報を表す。
// Valkeyキー: sess:{UUID}
// TTL: 24時間
type Session struct {
	UUID          string `json:"uuid"`            // セッション識別子
	IMSI          string `json:"imsi"`            // 加入者IMSI
	NasIP         string `json:"nas_ip"`          // NAS IPアドレス
	ClientIP      string `json:"client_ip"`       // クライアントIPアドレス
	AcctSessionID string `json:"acct_session_id"` // アカウンティングセッションID
	StartTime     int64  `json:"start_time"`      // セッション開始時刻（Unix秒）
	InputOctets   int64  `json:"input_octets"`    // 受信バイト数
	OutputOctets  int64  `json:"output_octets"`   // 送信バイト数
}

// NewSession は新しいSessionを生成する。
func NewSession(uuid, imsi, nasIP, clientIP, acctSessionID string, startTime int64) *Session {
	return &Session{
		UUID:          uuid,
		IMSI:          imsi,
		NasIP:         nasIP,
		ClientIP:      clientIP,
		AcctSessionID: acctSessionID,
		StartTime:     startTime,
		InputOctets:   0,
		OutputOctets:  0,
	}
}

// EAPContext はEAP認証コンテキストを表す。
// Valkeyキー: eap:{TraceID}
// TTL: 60秒
type EAPContext struct {
	TraceID              string `json:"trace_id"`               // トレース識別子
	IMSI                 string `json:"imsi"`                   // 加入者IMSI
	EAPType              uint8  `json:"eap_type"`               // EAPタイプ（23=AKA, 50=AKA'）
	Stage                Stage  `json:"stage"`                  // 認証ステージ
	RAND                 string `json:"rand"`                   // ランダム値（32文字16進数）
	AUTN                 string `json:"autn"`                   // 認証トークン（32文字16進数）
	XRES                 string `json:"xres"`                   // 期待される応答（16文字16進数）
	Kaut                 string `json:"kaut"`                   // 認証鍵
	MSK                  string `json:"msk"`                    // マスターセッションキー
	ResyncCount          int    `json:"resync_count"`           // 再同期試行回数
	PermanentIDRequested bool   `json:"permanent_id_requested"` // 永続ID要求フラグ
}

// NewEAPContext は新しいEAPContextを生成する。
func NewEAPContext(traceID, imsi string, eapType uint8) *EAPContext {
	return &EAPContext{
		TraceID:              traceID,
		IMSI:                 imsi,
		EAPType:              eapType,
		Stage:                StageNew,
		RAND:                 "",
		AUTN:                 "",
		XRES:                 "",
		Kaut:                 "",
		MSK:                  "",
		ResyncCount:          0,
		PermanentIDRequested: false,
	}
}
