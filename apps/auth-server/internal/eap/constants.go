package eap

// EAPコンテキストステージ
// Deprecated: EAPState型（statemachine.go）を使用してください。
const (
	// Deprecated: StateNew, StateWaitingIdentity, StateIdentityReceived を使用してください。
	StageIdentity = "identity"
	// Deprecated: StateChallengeSent を使用してください。
	StageChallenge = "challenge"
)

// Identity種別プレフィックス（先頭文字）
const (
	IdentityPrefixAKAPermanent      = '0' // EAP-AKA永続ID
	IdentityPrefixAKAPseudonym      = '2' // EAP-AKA仮名
	IdentityPrefixAKAReauth         = '4' // EAP-AKA再認証ID
	IdentityPrefixAKAPrimePermanent = '6' // EAP-AKA'永続ID
	IdentityPrefixAKAPrimePseudonym = '7' // EAP-AKA'仮名
	IdentityPrefixAKAPrimeReauth    = '8' // EAP-AKA'再認証ID
)

// EAP-SIM（非対応）
const (
	IdentityPrefixSIMPermanent = '1' // EAP-SIM永続ID
	IdentityPrefixSIMPseudonym = '3' // EAP-SIM仮名
	IdentityPrefixSIMReauth    = '5' // EAP-SIM再認証ID
)
