// Package validation はバリデーションルールを提供する。
package validation

import "regexp"

// バリデーション正規表現
var (
	// IMSIPattern はIMSI形式（15桁の数字）
	IMSIPattern = regexp.MustCompile(`^[0-9]{15}$`)

	// KiPattern はKi形式（32桁の16進数）
	KiPattern = regexp.MustCompile(`^[0-9A-Fa-f]{32}$`)

	// OPcPattern はOPc形式（32桁の16進数）
	OPcPattern = regexp.MustCompile(`^[0-9A-Fa-f]{32}$`)

	// AMFPattern はAMF形式（4桁の16進数）
	AMFPattern = regexp.MustCompile(`^[0-9A-Fa-f]{4}$`)

	// SQNPattern はSQN形式（12桁の16進数）
	SQNPattern = regexp.MustCompile(`^[0-9A-Fa-f]{12}$`)

	// IPv4Pattern はIPv4アドレス形式
	IPv4Pattern = regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)

	// SecretPattern はRADIUSシークレット形式（1-128文字のASCII印字可能文字）
	SecretPattern = regexp.MustCompile(`^[\x21-\x7E]{1,128}$`)

	// ClientNamePattern はクライアント名形式（1-64文字の英数字、ハイフン、アンダースコア）
	ClientNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

	// VendorPattern はベンダー名形式（0-64文字の英数字、スペース、ハイフン）
	VendorPattern = regexp.MustCompile(`^[a-zA-Z0-9 -]{0,64}$`)

	// NasIDPattern はNAS ID形式（1-253文字、ワイルドカード*可）
	NasIDPattern = regexp.MustCompile(`^[\x21-\x7E*]{1,253}$`)

	// SSIDPattern はSSID形式（1-32文字）
	SSIDPattern = regexp.MustCompile(`^.{1,32}$`)
)

// 定数
const (
	// MaxSecretLength はシークレットの最大長
	MaxSecretLength = 128
	// MaxClientNameLength はクライアント名の最大長
	MaxClientNameLength = 64
	// MaxVendorLength はベンダー名の最大長
	MaxVendorLength = 64
	// MaxSSIDLength はSSIDの最大長
	MaxSSIDLength = 32
	// MaxNasIDLength はNAS IDの最大長
	MaxNasIDLength = 253
	// MaxVlanID はVLAN IDの最大値
	MaxVlanID = 4094
	// MaxSessionTimeout はセッションタイムアウトの最大値（24時間）
	MaxSessionTimeout = 86400
)
