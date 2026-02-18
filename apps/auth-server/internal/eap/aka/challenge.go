package aka

import (
	"crypto/subtle"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap"
	eapaka "github.com/oyaguma3/go-eapaka"
)

// BuildChallenge はEAP-Request/AKA-Challengeパケットを構築する
// 属性: AT_RAND, AT_AUTN, AT_MAC
func BuildChallenge(identifier uint8, rand, autn, kAut []byte) ([]byte, error) {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeRequest,
		Identifier: identifier,
		Type:       eapaka.TypeAKA,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRand{Rand: rand},
			&eapaka.AtAutn{Autn: autn},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}

	// MAC計算・設定
	if err := pkt.CalculateAndSetMac(kAut); err != nil {
		return nil, err
	}

	return pkt.Marshal()
}

// VerifyChallengeResponse はEAP-Response/AKA-Challengeを検証する
// 検証順序: AT_MAC → AT_RES
func VerifyChallengeResponse(pkt *eapaka.Packet, kAut, xres []byte) error {
	// 1. MAC検証
	ok, err := pkt.VerifyMac(kAut)
	if err != nil {
		return eap.ErrMACInvalid
	}
	if !ok {
		return eap.ErrMACInvalid
	}

	// 2. AT_RES取得
	atRes, found := eap.GetAttribute[*eapaka.AtRes](pkt)
	if !found {
		return eap.ErrRESNotFound
	}

	// 3. RES長チェック
	if len(atRes.Res) != len(xres) {
		return eap.ErrRESLengthMismatch
	}

	// 4. RES値比較（タイミング攻撃対策）
	if subtle.ConstantTimeCompare(atRes.Res, xres) != 1 {
		return eap.ErrRESMismatch
	}

	return nil
}
