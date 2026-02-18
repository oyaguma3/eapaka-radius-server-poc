package akaprime

import (
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap"
	eapaka "github.com/oyaguma3/go-eapaka"
)

const testNetworkName = "WLAN"

// テスト用のヘルパー: 有効なChallenge/Responseペアに必要なパラメータを生成する
func setupAKAPrimeChallengeTest(t *testing.T) (kAut []byte, rand, autn, xres []byte) {
	t.Helper()

	identity := "6123456789012345@example.com"
	ck := make([]byte, 16)
	ik := make([]byte, 16)
	for i := range ck {
		ck[i] = byte(i + 1)
		ik[i] = byte(i + 17)
	}

	autn = make([]byte, 16)
	for i := range autn {
		autn[i] = byte(i + 100)
	}

	km, err := DeriveAllKeys(identity, ck, ik, autn, testNetworkName)
	if err != nil {
		t.Fatalf("DeriveAllKeys失敗: %v", err)
	}
	kAut = km.K_aut

	rand = make([]byte, 16)
	xres = make([]byte, 8)
	for i := range rand {
		rand[i] = byte(i + 200)
	}
	for i := range xres {
		xres[i] = byte(i + 50)
	}
	return
}

// buildValidAKAPrimeResponse はテスト用の有効なEAP-Response/AKA'-Challengeパケットを構築する
func buildValidAKAPrimeResponse(t *testing.T, kAut, xres []byte) *eapaka.Packet {
	t.Helper()

	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKAPrime,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRes{Res: xres},
			&eapaka.AtKdf{KDF: eapaka.KDFAKAPrimeWithCKIK},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}

	if err := pkt.CalculateAndSetMac(kAut); err != nil {
		t.Fatalf("MAC計算失敗: %v", err)
	}
	return pkt
}

func TestBuildChallenge_Success(t *testing.T) {
	kAut, rand, autn, _ := setupAKAPrimeChallengeTest(t)

	data, err := BuildChallenge(1, rand, autn, testNetworkName, kAut)
	if err != nil {
		t.Fatalf("BuildChallenge失敗: %v", err)
	}

	pkt, err := eapaka.Parse(data)
	if err != nil {
		t.Fatalf("パケットのパース失敗: %v", err)
	}
	if pkt.Subtype != eapaka.SubtypeChallenge {
		t.Errorf("Subtypeが不正: got=%d, want=%d", pkt.Subtype, eapaka.SubtypeChallenge)
	}
	if pkt.Type != eapaka.TypeAKAPrime {
		t.Errorf("Typeが不正: got=%d, want=%d", pkt.Type, eapaka.TypeAKAPrime)
	}
}

func TestBuildChallenge_ContainsKdfAttributes(t *testing.T) {
	kAut, rand, autn, _ := setupAKAPrimeChallengeTest(t)

	data, err := BuildChallenge(1, rand, autn, testNetworkName, kAut)
	if err != nil {
		t.Fatalf("BuildChallenge失敗: %v", err)
	}

	pkt, err := eapaka.Parse(data)
	if err != nil {
		t.Fatalf("パケットのパース失敗: %v", err)
	}

	// AT_KDF_INPUT存在確認
	atKdfInput, found := eap.GetAttribute[*eapaka.AtKdfInput](pkt)
	if !found {
		t.Fatal("AT_KDF_INPUTが見つからない")
	}
	if atKdfInput.NetworkName != testNetworkName {
		t.Errorf("NetworkNameが不正: got=%s, want=%s", atKdfInput.NetworkName, testNetworkName)
	}

	// AT_KDF存在確認
	kdfValues := eapaka.KdfValuesFromAttributes(pkt.Attributes)
	if len(kdfValues) != 1 {
		t.Fatalf("AT_KDFの数が不正: got=%d, want=1", len(kdfValues))
	}
	if kdfValues[0] != eapaka.KDFAKAPrimeWithCKIK {
		t.Errorf("KDF値が不正: got=%d, want=%d", kdfValues[0], eapaka.KDFAKAPrimeWithCKIK)
	}
}

func TestBuildChallenge_MACIsSet(t *testing.T) {
	kAut, rand, autn, _ := setupAKAPrimeChallengeTest(t)

	data, err := BuildChallenge(1, rand, autn, testNetworkName, kAut)
	if err != nil {
		t.Fatalf("BuildChallenge失敗: %v", err)
	}

	pkt, err := eapaka.Parse(data)
	if err != nil {
		t.Fatalf("パケットのパース失敗: %v", err)
	}

	atMac, found := eap.GetAttribute[*eapaka.AtMac](pkt)
	if !found {
		t.Fatal("AT_MACが見つからない")
	}

	allZero := true
	for _, b := range atMac.MAC {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("AT_MACが全ゼロ（MAC未計算の可能性）")
	}
}

func TestVerifyChallengeResponse_Success(t *testing.T) {
	kAut, _, _, xres := setupAKAPrimeChallengeTest(t)

	pkt := buildValidAKAPrimeResponse(t, kAut, xres)

	if err := VerifyChallengeResponse(pkt, kAut, xres); err != nil {
		t.Errorf("検証成功を期待したがエラー: %v", err)
	}
}

func TestVerifyChallengeResponse_KDFNotSupported(t *testing.T) {
	kAut, _, _, xres := setupAKAPrimeChallengeTest(t)

	// KDF=2（非サポート）のパケットを構築
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKAPrime,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRes{Res: xres},
			&eapaka.AtKdf{KDF: 2},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}
	if err := pkt.CalculateAndSetMac(kAut); err != nil {
		t.Fatalf("MAC計算失敗: %v", err)
	}

	err := VerifyChallengeResponse(pkt, kAut, xres)
	if !errors.Is(err, eap.ErrKDFNotSupported) {
		t.Errorf("ErrKDFNotSupportedを期待したが: %v", err)
	}
}

func TestVerifyChallengeResponse_KDFMultipleValues(t *testing.T) {
	kAut, _, _, xres := setupAKAPrimeChallengeTest(t)

	// AT_KDFが複数あるパケット
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKAPrime,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRes{Res: xres},
			&eapaka.AtKdf{KDF: eapaka.KDFAKAPrimeWithCKIK},
			&eapaka.AtKdf{KDF: 2},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}
	if err := pkt.CalculateAndSetMac(kAut); err != nil {
		t.Fatalf("MAC計算失敗: %v", err)
	}

	err := VerifyChallengeResponse(pkt, kAut, xres)
	if !errors.Is(err, eap.ErrKDFNotSupported) {
		t.Errorf("ErrKDFNotSupportedを期待したが: %v", err)
	}
}

func TestVerifyChallengeResponse_KDFAbsent(t *testing.T) {
	kAut, _, _, xres := setupAKAPrimeChallengeTest(t)

	// AT_KDFなしのパケット（受け入れ）
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKAPrime,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRes{Res: xres},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}
	if err := pkt.CalculateAndSetMac(kAut); err != nil {
		t.Fatalf("MAC計算失敗: %v", err)
	}

	err := VerifyChallengeResponse(pkt, kAut, xres)
	if err != nil {
		t.Errorf("AT_KDFなしは正常を期待したがエラー: %v", err)
	}
}

func TestVerifyChallengeResponse_MACInvalid(t *testing.T) {
	kAut, _, _, xres := setupAKAPrimeChallengeTest(t)

	pkt := buildValidAKAPrimeResponse(t, kAut, xres)

	// 不正なK_autで検証
	wrongKAut := make([]byte, 32)
	for i := range wrongKAut {
		wrongKAut[i] = 0xFF
	}

	err := VerifyChallengeResponse(pkt, wrongKAut, xres)
	if !errors.Is(err, eap.ErrMACInvalid) {
		t.Errorf("ErrMACInvalidを期待したが: %v", err)
	}
}

func TestVerifyChallengeResponse_RESNotFound(t *testing.T) {
	kAut, _, _, _ := setupAKAPrimeChallengeTest(t)

	// AT_RESなしのパケット
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKAPrime,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtKdf{KDF: eapaka.KDFAKAPrimeWithCKIK},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}
	if err := pkt.CalculateAndSetMac(kAut); err != nil {
		t.Fatalf("MAC計算失敗: %v", err)
	}

	err := VerifyChallengeResponse(pkt, kAut, make([]byte, 8))
	if !errors.Is(err, eap.ErrRESNotFound) {
		t.Errorf("ErrRESNotFoundを期待したが: %v", err)
	}
}

func TestVerifyChallengeResponse_RESLengthMismatch(t *testing.T) {
	kAut, _, _, _ := setupAKAPrimeChallengeTest(t)

	// 8バイトのRESを持つパケット
	res := make([]byte, 8)
	for i := range res {
		res[i] = byte(i + 50)
	}
	pkt := buildValidAKAPrimeResponse(t, kAut, res)

	// 異なる長さのxresで検証
	xres := make([]byte, 16)
	err := VerifyChallengeResponse(pkt, kAut, xres)
	if !errors.Is(err, eap.ErrRESLengthMismatch) {
		t.Errorf("ErrRESLengthMismatchを期待したが: %v", err)
	}
}

func TestVerifyChallengeResponse_VerifyMacError(t *testing.T) {
	kAut, _, _, xres := setupAKAPrimeChallengeTest(t)

	// AT_MACなし・AT_KDFなし（KDF検証パス）のパケットでVerifyMacエラーを誘発
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKAPrime,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRes{Res: xres},
		},
	}

	err := VerifyChallengeResponse(pkt, kAut, xres)
	if !errors.Is(err, eap.ErrMACInvalid) {
		t.Errorf("ErrMACInvalidを期待したが: %v", err)
	}
}

func TestVerifyChallengeResponse_RESMismatch(t *testing.T) {
	kAut, _, _, xres := setupAKAPrimeChallengeTest(t)

	pkt := buildValidAKAPrimeResponse(t, kAut, xres)

	// 異なる値のxresで検証
	wrongXres := make([]byte, len(xres))
	for i := range wrongXres {
		wrongXres[i] = 0xFF
	}

	err := VerifyChallengeResponse(pkt, kAut, wrongXres)
	if !errors.Is(err, eap.ErrRESMismatch) {
		t.Errorf("ErrRESMismatchを期待したが: %v", err)
	}
}
