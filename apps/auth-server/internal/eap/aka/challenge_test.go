package aka

import (
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap"
	eapaka "github.com/oyaguma3/go-eapaka"
)

// テスト用のヘルパー: 有効なChallenge/Responseペアを生成する
func setupChallengeTest(t *testing.T) (kAut, rand, autn, xres []byte) {
	t.Helper()

	identity := "0123456789012345@example.com"
	ck := make([]byte, 16)
	ik := make([]byte, 16)
	for i := range ck {
		ck[i] = byte(i + 1)
		ik[i] = byte(i + 17)
	}

	km := DeriveKeys(identity, ck, ik)
	kAut = km.K_aut

	rand = make([]byte, 16)
	autn = make([]byte, 16)
	xres = make([]byte, 8)
	for i := range rand {
		rand[i] = byte(i + 100)
		autn[i] = byte(i + 200)
	}
	for i := range xres {
		xres[i] = byte(i + 50)
	}
	return
}

// buildValidResponse はテスト用の有効なEAP-Response/AKA-Challengeパケットを構築する
func buildValidResponse(t *testing.T, kAut, xres []byte) *eapaka.Packet {
	t.Helper()

	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKA,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
			&eapaka.AtRes{Res: xres},
			&eapaka.AtMac{MAC: make([]byte, 16)},
		},
	}

	if err := pkt.CalculateAndSetMac(kAut); err != nil {
		t.Fatalf("MAC計算失敗: %v", err)
	}
	return pkt
}

func TestBuildChallenge_Success(t *testing.T) {
	kAut, rand, autn, _ := setupChallengeTest(t)

	data, err := BuildChallenge(1, rand, autn, kAut)
	if err != nil {
		t.Fatalf("BuildChallenge失敗: %v", err)
	}

	// 再パースして検証
	pkt, err := eapaka.Parse(data)
	if err != nil {
		t.Fatalf("パケットのパース失敗: %v", err)
	}
	if pkt.Subtype != eapaka.SubtypeChallenge {
		t.Errorf("Subtypeが不正: got=%d, want=%d", pkt.Subtype, eapaka.SubtypeChallenge)
	}
	if pkt.Code != eapaka.CodeRequest {
		t.Errorf("Codeが不正: got=%d, want=%d", pkt.Code, eapaka.CodeRequest)
	}
}

func TestBuildChallenge_ContainsAttributes(t *testing.T) {
	kAut, rand, autn, _ := setupChallengeTest(t)

	data, err := BuildChallenge(1, rand, autn, kAut)
	if err != nil {
		t.Fatalf("BuildChallenge失敗: %v", err)
	}

	pkt, err := eapaka.Parse(data)
	if err != nil {
		t.Fatalf("パケットのパース失敗: %v", err)
	}

	// AT_RAND存在確認
	if _, found := eap.GetAttribute[*eapaka.AtRand](pkt); !found {
		t.Error("AT_RANDが見つからない")
	}
	// AT_AUTN存在確認
	if _, found := eap.GetAttribute[*eapaka.AtAutn](pkt); !found {
		t.Error("AT_AUTNが見つからない")
	}
	// AT_MAC存在確認
	if _, found := eap.GetAttribute[*eapaka.AtMac](pkt); !found {
		t.Error("AT_MACが見つからない")
	}
}

func TestBuildChallenge_MACIsSet(t *testing.T) {
	kAut, rand, autn, _ := setupChallengeTest(t)

	data, err := BuildChallenge(1, rand, autn, kAut)
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
	kAut, _, _, xres := setupChallengeTest(t)

	pkt := buildValidResponse(t, kAut, xres)

	if err := VerifyChallengeResponse(pkt, kAut, xres); err != nil {
		t.Errorf("検証成功を期待したがエラー: %v", err)
	}
}

func TestVerifyChallengeResponse_MACInvalid(t *testing.T) {
	kAut, _, _, xres := setupChallengeTest(t)

	pkt := buildValidResponse(t, kAut, xres)

	// 不正なK_autで検証
	wrongKAut := make([]byte, 16)
	for i := range wrongKAut {
		wrongKAut[i] = 0xFF
	}

	err := VerifyChallengeResponse(pkt, wrongKAut, xres)
	if !errors.Is(err, eap.ErrMACInvalid) {
		t.Errorf("ErrMACInvalidを期待したが: %v", err)
	}
}

func TestVerifyChallengeResponse_RESNotFound(t *testing.T) {
	kAut, _, _, _ := setupChallengeTest(t)

	// AT_RESなしのパケットを構築
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKA,
		Subtype:    eapaka.SubtypeChallenge,
		Attributes: []eapaka.Attribute{
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
	kAut, _, _, _ := setupChallengeTest(t)

	// 8バイトのRESを持つパケットを構築
	res := make([]byte, 8)
	for i := range res {
		res[i] = byte(i + 50)
	}
	pkt := buildValidResponse(t, kAut, res)

	// 異なる長さのxresで検証
	xres := make([]byte, 16) // 長さ不一致
	err := VerifyChallengeResponse(pkt, kAut, xres)
	if !errors.Is(err, eap.ErrRESLengthMismatch) {
		t.Errorf("ErrRESLengthMismatchを期待したが: %v", err)
	}
}

func TestVerifyChallengeResponse_VerifyMacError(t *testing.T) {
	kAut, _, _, xres := setupChallengeTest(t)

	// AT_MACを含まないパケットを構築してVerifyMacエラーを誘発
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKA,
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
	kAut, _, _, xres := setupChallengeTest(t)

	pkt := buildValidResponse(t, kAut, xres)

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
