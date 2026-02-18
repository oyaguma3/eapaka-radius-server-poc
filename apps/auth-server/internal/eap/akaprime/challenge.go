package akaprime

import (
	"crypto/subtle"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap"
	eapaka "github.com/oyaguma3/go-eapaka"
)

// BuildChallenge はEAP-Request/AKA'-Challengeパケットを構築する
// 属性: AT_RAND, AT_AUTN, AT_KDF_INPUT, AT_KDF, AT_MAC
func BuildChallenge(identifier uint8, rand, autn []byte, networkName string, kAut []byte) ([]byte, error) {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeRequest,
		Identifier: identifier,
		Type:       eapaka.TypeAKAPrime,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRand{Rand: rand},
			&eapaka.AtAutn{Autn: autn},
			&eapaka.AtKdfInput{NetworkName: networkName},
			&eapaka.AtKdf{KDF: eapaka.KDFAKAPrimeWithCKIK},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}

	// MAC計算・設定
	if err := pkt.CalculateAndSetMac(kAut); err != nil {
		return nil, err
	}

	return pkt.Marshal()
}

// VerifyChallengeResponse はEAP-Response/AKA'-Challengeを検証する
// 検証順序: AT_KDF → AT_MAC → AT_RES
func VerifyChallengeResponse(pkt *eapaka.Packet, kAut, xres []byte) error {
	// 1. KDF検証
	if err := validateKdfInResponse(pkt); err != nil {
		return err
	}

	// 2. MAC検証
	ok, err := pkt.VerifyMac(kAut)
	if err != nil {
		return eap.ErrMACInvalid
	}
	if !ok {
		return eap.ErrMACInvalid
	}

	// 3. AT_RES取得
	atRes, found := eap.GetAttribute[*eapaka.AtRes](pkt)
	if !found {
		return eap.ErrRESNotFound
	}

	// 4. RES長チェック
	if len(atRes.Res) != len(xres) {
		return eap.ErrRESLengthMismatch
	}

	// 5. RES値比較（タイミング攻撃対策）
	if subtle.ConstantTimeCompare(atRes.Res, xres) != 1 {
		return eap.ErrRESMismatch
	}

	return nil
}

// validateKdfInResponse はAT_KDFの検証を行う
func validateKdfInResponse(pkt *eapaka.Packet) error {
	kdfValues := eapaka.KdfValuesFromAttributes(pkt.Attributes)

	// AT_KDFなし → 正常（受け入れ）
	if len(kdfValues) == 0 {
		return nil
	}

	// KDF値が1つでKDF=1 → 正常
	if len(kdfValues) == 1 && kdfValues[0] == eapaka.KDFAKAPrimeWithCKIK {
		return nil
	}

	// それ以外 → 非サポート
	return eap.ErrKDFNotSupported
}
