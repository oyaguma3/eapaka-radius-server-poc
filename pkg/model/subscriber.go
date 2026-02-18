// Package model は共通データ構造体を提供する。
package model

// Subscriber は加入者情報を表す。
// Valkeyキー: sub:{IMSI}
type Subscriber struct {
	IMSI      string `json:"imsi"`       // 国際移動体加入者識別番号（15桁）
	Ki        string `json:"ki"`         // 秘密鍵（32文字16進数）
	OPc       string `json:"opc"`        // オペレータ定数（32文字16進数）
	AMF       string `json:"amf"`        // 認証管理フィールド（4文字16進数）
	SQN       string `json:"sqn"`        // シーケンス番号（12文字16進数）
	CreatedAt string `json:"created_at"` // 作成日時（RFC3339形式）
}

// NewSubscriber は新しいSubscriberを生成する。
func NewSubscriber(imsi, ki, opc, amf, sqn, createdAt string) *Subscriber {
	return &Subscriber{
		IMSI:      imsi,
		Ki:        ki,
		OPc:       opc,
		AMF:       amf,
		SQN:       sqn,
		CreatedAt: createdAt,
	}
}
