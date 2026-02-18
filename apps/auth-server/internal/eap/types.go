package eap

import "context"

// Action はEAP処理結果のアクション種別を表す
type Action string

const (
	ActionAccept    Action = "ACCEPT"
	ActionReject    Action = "REJECT"
	ActionChallenge Action = "CHALLENGE"
	ActionDrop      Action = "DROP"
)

// Request はEAP処理への入力を表す
type Request struct {
	TraceID       string // リクエスト追跡用UUID
	SrcIP         string // 送信元IPアドレス
	NASIdentifier string // NAS-Identifier属性
	CalledStation string // Called-Station-Id属性
	UserName      string // User-Name属性
	State         []byte // RADIUS State属性（TraceID格納）
	EAPMessage    []byte // EAP-Messageバイト列
}

// Result はEAP処理の結果を表す
type Result struct {
	Action         Action // 応答アクション
	EAPMessage     []byte // 応答EAPメッセージ
	State          []byte // Challenge時: []byte(TraceID)
	IMSI           string // 認証対象のIMSI
	SessionID      string // Accept時: セッションID
	MSK            []byte // Accept時: Master Session Key
	VlanID         string // Accept時: VLAN ID
	SessionTimeout int    // Accept時: セッションタイムアウト秒数
}

// EAPProcessor はEAP認証処理のインターフェース
type EAPProcessor interface {
	Process(ctx context.Context, req *Request) (*Result, error)
}
