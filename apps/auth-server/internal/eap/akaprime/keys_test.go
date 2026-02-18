package akaprime

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// hexDecode はテスト用にhex文字列をバイト列に変換する
func hexDecode(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex decode失敗: %q: %v", s, err)
	}
	return b
}

// rfc5448TestVector はRFC 5448 Appendix Cのテストベクターを表す
type rfc5448TestVector struct {
	identity    string
	networkName string
	ck          string
	ik          string
	autn        string
	ckPrime     string
	ikPrime     string
	kEncr       string
	kAut        string
	kRe         string
	msk         string
	emsk        string
}

// RFC 5448 Appendix C テストベクター
var rfc5448Vectors = []rfc5448TestVector{
	{ // Case 1: Milenage test set 19, WLAN
		identity:    "0555444333222111",
		networkName: "WLAN",
		ck:          "5349fbe098649f948f5d2e973a81c00f",
		ik:          "9744871ad32bf9bbd1dd5ce54e3e2e5a",
		autn:        "bb52e91c747ac3ab2a5c23d15ee351d5",
		ckPrime:     "0093962d0dd84aa5684b045c9edffa04",
		ikPrime:     "ccfc230ca74fcc96c0a5d61164f5a76c",
		kEncr:       "766fa0a6c317174b812d52fbcd11a179",
		kAut:        "0842ea722ff6835bfa2032499fc3ec23c2f0e388b4f07543ffc677f1696d71ea",
		kRe:         "cf83aa8bc7e0aced892acc98e76a9b2095b558c7795c7094715cb3393aa7d17a",
		msk:         "67c42d9aa56c1b79e295e3459fc3d187d42be0bf818d3070e362c5e967a4d544e8ecfe19358ab3039aff03b7c930588c055babee58a02650b067ec4e9347c75a",
		emsk:        "f861703cd775590e16c7679ea3874ada866311de290764d760cf76df647ea01c313f69924bdd7650ca9bac141ea075c4ef9e8029c0e290cdbad5638b63bc23fb",
	},
	{ // Case 2: Milenage test set 19, HRPD
		identity:    "0555444333222111",
		networkName: "HRPD",
		ck:          "5349fbe098649f948f5d2e973a81c00f",
		ik:          "9744871ad32bf9bbd1dd5ce54e3e2e5a",
		autn:        "bb52e91c747ac3ab2a5c23d15ee351d5",
		ckPrime:     "3820f0277fa5f77732b1fb1d90c1a0da",
		ikPrime:     "db94a0ab557ef6c9ab48619ca05b9a9f",
		kEncr:       "05ad73ac915fce89ac77e1520d82187b",
		kAut:        "5b4acaef62c6ebb8882b2f3d534c4b35277337a00184f20ff25d224c04be2afd",
		kRe:         "3f90bf5c6e5ef325ff04eb5ef6539fa8cca8398194fbd00be425b3f40dba10ac",
		msk:         "87b321570117cd6c95ab6c436fb5073ff15cf85505d2bc5bb7355fc21ea8a75757e8f86a2b138002e05752913bb43b82f868a96117e91a2d95f526677d572900",
		emsk:        "c891d5f20f148a1007553e2dea555c9cb672e9675f4a66b4bafa027379f93aee539a5979d0a0042b9d2ae28bed3b17a31dc8ab75072b80bd0c1da612466e402c",
	},
	{ // Case 3: Artificial values, WLAN
		identity:    "0555444333222111",
		networkName: "WLAN",
		ck:          "c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0",
		ik:          "b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0",
		autn:        "a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0",
		ckPrime:     "cd4c8e5c68f57dd1d7d7dfd0c538e577",
		ikPrime:     "3ece6b705dbbf7dfc459a11280c65524",
		kEncr:       "897d302fa2847416488c28e20dcb7be4",
		kAut:        "c40700e7722483ae3dc7139eb0b88bb558cb3081eccd057f9207d1286ee7dd53",
		kRe:         "0a591a22dd8b5b1cf29e3d508c91dbbdb4aee23051892c42b6a2de66ea504473",
		msk:         "9f7dca9e37bb22029ed986e7cd09d4a70d1ac76d95535c5cac40a7504699bb8961a29ef6f3e90f183de5861ad1bedc81ce9916391b401aa006c98785a5756df7",
		emsk:        "724de00bdb9e568187be3fe746114557d5018779537ee37f4d3c6c738cb97b9dc651bc19bfadc344ffe2b52ca78bd8316b51dacc5f2b1440cb9515521cc7ba23",
	},
	{ // Case 4: Artificial values, HRPD
		identity:    "0555444333222111",
		networkName: "HRPD",
		ck:          "c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0",
		ik:          "b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0",
		autn:        "a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0",
		ckPrime:     "8310a71ce6f754889613da8f64d5fb46",
		ikPrime:     "5adf14360ae838192db23f6fcb7f8c76",
		kEncr:       "745e7439ba238f50fcac4d15d47cd1d9",
		kAut:        "3e1d2aa4e677025cfd862a4be18361a13a645765571463df833a9759e8099879",
		kRe:         "99da835e2ae82462576fe6516fad1f802f0fa1191655dd0a273da96d04e0fcd3",
		msk:         "c6d3a6e0ceea951eb20d74f32c3061d0680a04b0b086ee8700ace3e0b95fa02683c287beee44432294ff98af26d2cc783bace75c4b0af7fdfeb5511ba8e4cbd0",
		emsk:        "7fb56813838adafa99d140c2f198f6dacebfb6afee444961105402b508c7f363352cb2919644b50463e6a69354150147ae09cbc54b8a651d8787a6893ed8536d",
	},
}

func testCKIK(t *testing.T) (ck, ik []byte) {
	t.Helper()
	ck = make([]byte, 16)
	ik = make([]byte, 16)
	for i := range ck {
		ck[i] = byte(i + 1)
		ik[i] = byte(i + 17)
	}
	return
}

func testAUTN(t *testing.T) []byte {
	t.Helper()
	autn := make([]byte, 16)
	for i := range autn {
		autn[i] = byte(i + 100)
	}
	return autn
}

func TestDeriveCKPrimeIKPrime_ValidInput(t *testing.T) {
	ck, ik := testCKIK(t)
	autn := testAUTN(t)
	networkName := "WLAN"

	ckPrime, ikPrime, err := DeriveCKPrimeIKPrime(ck, ik, networkName, autn)
	if err != nil {
		t.Fatalf("DeriveCKPrimeIKPrime失敗: %v", err)
	}
	if len(ckPrime) != 16 {
		t.Errorf("CK'のサイズが不正: got=%d, want=16", len(ckPrime))
	}
	if len(ikPrime) != 16 {
		t.Errorf("IK'のサイズが不正: got=%d, want=16", len(ikPrime))
	}
}

func TestDeriveCKPrimeIKPrime_DifferentNetworkName(t *testing.T) {
	ck, ik := testCKIK(t)
	autn := testAUTN(t)

	ckPrime1, ikPrime1, err := DeriveCKPrimeIKPrime(ck, ik, "WLAN", autn)
	if err != nil {
		t.Fatalf("DeriveCKPrimeIKPrime失敗: %v", err)
	}

	ckPrime2, ikPrime2, err := DeriveCKPrimeIKPrime(ck, ik, "5G:mnc001.mcc001.3gppnetwork.org", autn)
	if err != nil {
		t.Fatalf("DeriveCKPrimeIKPrime失敗: %v", err)
	}

	if bytes.Equal(ckPrime1, ckPrime2) {
		t.Error("異なるネットワーク名で同一のCK'が生成された")
	}
	if bytes.Equal(ikPrime1, ikPrime2) {
		t.Error("異なるネットワーク名で同一のIK'が生成された")
	}
}

func TestDeriveKeys_ValidInput(t *testing.T) {
	ck, ik := testCKIK(t)
	autn := testAUTN(t)

	ckPrime, ikPrime, err := DeriveCKPrimeIKPrime(ck, ik, "WLAN", autn)
	if err != nil {
		t.Fatalf("DeriveCKPrimeIKPrime失敗: %v", err)
	}

	km := DeriveKeys("6123456789012345@example.com", ckPrime, ikPrime)

	if len(km.K_encr) != 16 {
		t.Errorf("K_encrのサイズが不正: got=%d, want=16", len(km.K_encr))
	}
	if len(km.K_aut) != 32 {
		t.Errorf("K_autのサイズが不正: got=%d, want=32", len(km.K_aut))
	}
	if len(km.K_re) != 32 {
		t.Errorf("K_reのサイズが不正: got=%d, want=32", len(km.K_re))
	}
	if len(km.MSK) != 64 {
		t.Errorf("MSKのサイズが不正: got=%d, want=64", len(km.MSK))
	}
	if len(km.EMSK) != 64 {
		t.Errorf("EMSKのサイズが不正: got=%d, want=64", len(km.EMSK))
	}
}

func TestDeriveKeys_KautLength(t *testing.T) {
	ck, ik := testCKIK(t)
	autn := testAUTN(t)

	ckPrime, ikPrime, err := DeriveCKPrimeIKPrime(ck, ik, "WLAN", autn)
	if err != nil {
		t.Fatalf("DeriveCKPrimeIKPrime失敗: %v", err)
	}

	km := DeriveKeys("6123456789012345@example.com", ckPrime, ikPrime)

	// EAP-AKA'のK_autは32バイト（EAP-AKAの16バイトと異なる）
	if len(km.K_aut) != 32 {
		t.Errorf("K_autは32バイトであるべき: got=%d", len(km.K_aut))
	}
}

func TestDeriveAllKeys_Success(t *testing.T) {
	ck, ik := testCKIK(t)
	autn := testAUTN(t)
	identity := "6123456789012345@example.com"
	networkName := "WLAN"

	km, err := DeriveAllKeys(identity, ck, ik, autn, networkName)
	if err != nil {
		t.Fatalf("DeriveAllKeys失敗: %v", err)
	}

	if len(km.K_encr) != 16 {
		t.Errorf("K_encrのサイズが不正: got=%d, want=16", len(km.K_encr))
	}
	if len(km.K_aut) != 32 {
		t.Errorf("K_autのサイズが不正: got=%d, want=32", len(km.K_aut))
	}
	if len(km.K_re) != 32 {
		t.Errorf("K_reのサイズが不正: got=%d, want=32", len(km.K_re))
	}
	if len(km.MSK) != 64 {
		t.Errorf("MSKのサイズが不正: got=%d, want=64", len(km.MSK))
	}
	if len(km.EMSK) != 64 {
		t.Errorf("EMSKのサイズが不正: got=%d, want=64", len(km.EMSK))
	}
}

// RFC 5448 Appendix C テストベクターによるKAT（Known Answer Test）

func TestDeriveCKPrimeIKPrime_RFC5448(t *testing.T) {
	for i, v := range rfc5448Vectors {
		t.Run(v.networkName, func(t *testing.T) {
			ck := hexDecode(t, v.ck)
			ik := hexDecode(t, v.ik)
			autn := hexDecode(t, v.autn)

			ckPrime, ikPrime, err := DeriveCKPrimeIKPrime(ck, ik, v.networkName, autn)
			if err != nil {
				t.Fatalf("Case %d: DeriveCKPrimeIKPrime失敗: %v", i+1, err)
			}

			wantCKPrime := hexDecode(t, v.ckPrime)
			wantIKPrime := hexDecode(t, v.ikPrime)

			if !bytes.Equal(ckPrime, wantCKPrime) {
				t.Errorf("Case %d: CK'不一致\n  got:  %x\n  want: %x", i+1, ckPrime, wantCKPrime)
			}
			if !bytes.Equal(ikPrime, wantIKPrime) {
				t.Errorf("Case %d: IK'不一致\n  got:  %x\n  want: %x", i+1, ikPrime, wantIKPrime)
			}
		})
	}
}

func TestDeriveKeys_RFC5448(t *testing.T) {
	for i, v := range rfc5448Vectors {
		t.Run(v.networkName, func(t *testing.T) {
			ckPrime := hexDecode(t, v.ckPrime)
			ikPrime := hexDecode(t, v.ikPrime)

			km := DeriveKeys(v.identity, ckPrime, ikPrime)

			wantKEncr := hexDecode(t, v.kEncr)
			wantKAut := hexDecode(t, v.kAut)
			wantKRe := hexDecode(t, v.kRe)
			wantMSK := hexDecode(t, v.msk)
			wantEMSK := hexDecode(t, v.emsk)

			if !bytes.Equal(km.K_encr, wantKEncr) {
				t.Errorf("Case %d: K_encr不一致\n  got:  %x\n  want: %x", i+1, km.K_encr, wantKEncr)
			}
			if !bytes.Equal(km.K_aut, wantKAut) {
				t.Errorf("Case %d: K_aut不一致\n  got:  %x\n  want: %x", i+1, km.K_aut, wantKAut)
			}
			if !bytes.Equal(km.K_re, wantKRe) {
				t.Errorf("Case %d: K_re不一致\n  got:  %x\n  want: %x", i+1, km.K_re, wantKRe)
			}
			if !bytes.Equal(km.MSK, wantMSK) {
				t.Errorf("Case %d: MSK不一致\n  got:  %x\n  want: %x", i+1, km.MSK, wantMSK)
			}
			if !bytes.Equal(km.EMSK, wantEMSK) {
				t.Errorf("Case %d: EMSK不一致\n  got:  %x\n  want: %x", i+1, km.EMSK, wantEMSK)
			}
		})
	}
}

func TestDeriveAllKeys_RFC5448(t *testing.T) {
	for i, v := range rfc5448Vectors {
		t.Run(v.networkName, func(t *testing.T) {
			ck := hexDecode(t, v.ck)
			ik := hexDecode(t, v.ik)
			autn := hexDecode(t, v.autn)

			km, err := DeriveAllKeys(v.identity, ck, ik, autn, v.networkName)
			if err != nil {
				t.Fatalf("Case %d: DeriveAllKeys失敗: %v", i+1, err)
			}

			wantKEncr := hexDecode(t, v.kEncr)
			wantKAut := hexDecode(t, v.kAut)
			wantKRe := hexDecode(t, v.kRe)
			wantMSK := hexDecode(t, v.msk)
			wantEMSK := hexDecode(t, v.emsk)

			if !bytes.Equal(km.K_encr, wantKEncr) {
				t.Errorf("Case %d: K_encr不一致\n  got:  %x\n  want: %x", i+1, km.K_encr, wantKEncr)
			}
			if !bytes.Equal(km.K_aut, wantKAut) {
				t.Errorf("Case %d: K_aut不一致\n  got:  %x\n  want: %x", i+1, km.K_aut, wantKAut)
			}
			if !bytes.Equal(km.K_re, wantKRe) {
				t.Errorf("Case %d: K_re不一致\n  got:  %x\n  want: %x", i+1, km.K_re, wantKRe)
			}
			if !bytes.Equal(km.MSK, wantMSK) {
				t.Errorf("Case %d: MSK不一致\n  got:  %x\n  want: %x", i+1, km.MSK, wantMSK)
			}
			if !bytes.Equal(km.EMSK, wantEMSK) {
				t.Errorf("Case %d: EMSK不一致\n  got:  %x\n  want: %x", i+1, km.EMSK, wantEMSK)
			}
		})
	}
}

func TestDeriveAllKeys_DeriveCKPrimeIKPrimeError(t *testing.T) {
	tests := []struct {
		name        string
		ck          []byte
		ik          []byte
		autn        []byte
		networkName string
	}{
		{
			name:        "CKが15バイト",
			ck:          make([]byte, 15),
			ik:          make([]byte, 16),
			autn:        make([]byte, 16),
			networkName: "WLAN",
		},
		{
			name:        "IKが17バイト",
			ck:          make([]byte, 16),
			ik:          make([]byte, 17),
			autn:        make([]byte, 16),
			networkName: "WLAN",
		},
		{
			name:        "AUTNが5バイト",
			ck:          make([]byte, 16),
			ik:          make([]byte, 16),
			autn:        make([]byte, 5),
			networkName: "WLAN",
		},
		{
			name:        "networkNameが空文字列",
			ck:          make([]byte, 16),
			ik:          make([]byte, 16),
			autn:        make([]byte, 16),
			networkName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DeriveAllKeys("test@example.com", tt.ck, tt.ik, tt.autn, tt.networkName)
			if err == nil {
				t.Error("エラーを期待したがnilが返された")
			}
		})
	}
}

func TestDeriveAllKeys_Deterministic(t *testing.T) {
	ck, ik := testCKIK(t)
	autn := testAUTN(t)
	identity := "6123456789012345@example.com"
	networkName := "WLAN"

	km1, err := DeriveAllKeys(identity, ck, ik, autn, networkName)
	if err != nil {
		t.Fatalf("DeriveAllKeys失敗: %v", err)
	}

	km2, err := DeriveAllKeys(identity, ck, ik, autn, networkName)
	if err != nil {
		t.Fatalf("DeriveAllKeys失敗: %v", err)
	}

	if !bytes.Equal(km1.K_encr, km2.K_encr) {
		t.Error("同一入力で異なるK_encrが生成された")
	}
	if !bytes.Equal(km1.K_aut, km2.K_aut) {
		t.Error("同一入力で異なるK_autが生成された")
	}
	if !bytes.Equal(km1.K_re, km2.K_re) {
		t.Error("同一入力で異なるK_reが生成された")
	}
	if !bytes.Equal(km1.MSK, km2.MSK) {
		t.Error("同一入力で異なるMSKが生成された")
	}
	if !bytes.Equal(km1.EMSK, km2.EMSK) {
		t.Error("同一入力で異なるEMSKが生成された")
	}
}
