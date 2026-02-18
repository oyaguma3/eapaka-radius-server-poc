package eap

import (
	"strings"

	eapaka "github.com/oyaguma3/go-eapaka"
)

// IdentityType はIdentityの種別を表す
type IdentityType int

const (
	IdentityTypePermanentAKA      IdentityType = iota // '0' EAP-AKA永続ID
	IdentityTypePseudonymAKA                          // '2' EAP-AKA仮名
	IdentityTypeReauthAKA                             // '4' EAP-AKA再認証ID
	IdentityTypePermanentAKAPrime                     // '6' EAP-AKA'永続ID
	IdentityTypePseudonymAKAPrime                     // '7' EAP-AKA'仮名
	IdentityTypeReauthAKAPrime                        // '8' EAP-AKA'再認証ID
	IdentityTypeUnsupported                           // EAP-SIM等の非対応種別
	IdentityTypeInvalid                               // 不正形式
)

// ParsedIdentity はパース済みのIdentity情報を保持する
type ParsedIdentity struct {
	Type    IdentityType // Identity種別
	IMSI    string       // 永続IDの場合のIMSI（先頭プレフィックス除く）
	Raw     string       // 元のIdentity文字列
	Realm   string       // @以降の部分
	EAPType uint8        // eapaka.TypeAKA(23) or eapaka.TypeAKAPrime(50)
}

// ParseIdentity はIdentity文字列を解析してParsedIdentityを返す
func ParseIdentity(identity string) (*ParsedIdentity, error) {
	if identity == "" {
		return nil, ErrInvalidIdentity
	}

	// @でRealmを分離
	userPart, realm, found := strings.Cut(identity, "@")
	if !found {
		return nil, ErrMissingRealm
	}

	// userPartが空（"@realm"形式）
	if userPart == "" {
		return nil, ErrInvalidIdentity
	}

	prefix := byte(userPart[0])
	parsed := &ParsedIdentity{
		Raw:   identity,
		Realm: realm,
	}

	switch prefix {
	case byte(IdentityPrefixAKAPermanent): // '0'
		parsed.Type = IdentityTypePermanentAKA
		parsed.IMSI = userPart[1:]
		parsed.EAPType = eapaka.TypeAKA
	case byte(IdentityPrefixAKAPseudonym): // '2'
		parsed.Type = IdentityTypePseudonymAKA
		parsed.EAPType = eapaka.TypeAKA
	case byte(IdentityPrefixAKAReauth): // '4'
		parsed.Type = IdentityTypeReauthAKA
		parsed.EAPType = eapaka.TypeAKA
	case byte(IdentityPrefixAKAPrimePermanent): // '6'
		parsed.Type = IdentityTypePermanentAKAPrime
		parsed.IMSI = userPart[1:]
		parsed.EAPType = eapaka.TypeAKAPrime
	case byte(IdentityPrefixAKAPrimePseudonym): // '7'
		parsed.Type = IdentityTypePseudonymAKAPrime
		parsed.EAPType = eapaka.TypeAKAPrime
	case byte(IdentityPrefixAKAPrimeReauth): // '8'
		parsed.Type = IdentityTypeReauthAKAPrime
		parsed.EAPType = eapaka.TypeAKAPrime
	case byte(IdentityPrefixSIMPermanent), // '1'
		byte(IdentityPrefixSIMPseudonym), // '3'
		byte(IdentityPrefixSIMReauth):    // '5'
		return nil, ErrUnsupportedIdentity
	default:
		return nil, ErrInvalidIdentity
	}

	return parsed, nil
}

// RequiresFullAuth は仮名IDまたは再認証IDの場合にtrueを返す
// フル認証への誘導が必要かどうかの判定に使用する
func (p *ParsedIdentity) RequiresFullAuth() bool {
	switch p.Type {
	case IdentityTypePseudonymAKA,
		IdentityTypeReauthAKA,
		IdentityTypePseudonymAKAPrime,
		IdentityTypeReauthAKAPrime:
		return true
	default:
		return false
	}
}

// IsPermanent は永続ID（'0' or '6'）かどうかを判定する
func (p *ParsedIdentity) IsPermanent() bool {
	return p.Type == IdentityTypePermanentAKA || p.Type == IdentityTypePermanentAKAPrime
}

// IsAKAPrime はEAP-AKA'方式（'6','7','8'）かどうかを判定する
func (p *ParsedIdentity) IsAKAPrime() bool {
	return p.Type == IdentityTypePermanentAKAPrime ||
		p.Type == IdentityTypePseudonymAKAPrime ||
		p.Type == IdentityTypeReauthAKAPrime
}
