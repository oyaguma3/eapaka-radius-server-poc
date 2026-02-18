package aka

import (
	"bytes"
	"testing"
)

func TestDeriveKeys_ValidInput(t *testing.T) {
	identity := "0123456789012345@example.com"
	ck := make([]byte, 16)
	ik := make([]byte, 16)
	for i := range ck {
		ck[i] = byte(i)
		ik[i] = byte(i + 16)
	}

	km := DeriveKeys(identity, ck, ik)

	if len(km.K_encr) != 16 {
		t.Errorf("K_encrのサイズが不正: got=%d, want=16", len(km.K_encr))
	}
	if len(km.K_aut) != 16 {
		t.Errorf("K_autのサイズが不正: got=%d, want=16", len(km.K_aut))
	}
	if len(km.MSK) != 64 {
		t.Errorf("MSKのサイズが不正: got=%d, want=64", len(km.MSK))
	}
	if len(km.EMSK) != 64 {
		t.Errorf("EMSKのサイズが不正: got=%d, want=64", len(km.EMSK))
	}
}

func TestDeriveKeys_DifferentIdentity(t *testing.T) {
	ck := make([]byte, 16)
	ik := make([]byte, 16)
	for i := range ck {
		ck[i] = byte(i)
		ik[i] = byte(i + 16)
	}

	km1 := DeriveKeys("user1@example.com", ck, ik)
	km2 := DeriveKeys("user2@example.com", ck, ik)

	if bytes.Equal(km1.K_encr, km2.K_encr) {
		t.Error("異なるidentityで同一のK_encrが生成された")
	}
	if bytes.Equal(km1.K_aut, km2.K_aut) {
		t.Error("異なるidentityで同一のK_autが生成された")
	}
	if bytes.Equal(km1.MSK, km2.MSK) {
		t.Error("異なるidentityで同一のMSKが生成された")
	}
	if bytes.Equal(km1.EMSK, km2.EMSK) {
		t.Error("異なるidentityで同一のEMSKが生成された")
	}
}

func TestDeriveKeys_DifferentCKIK(t *testing.T) {
	identity := "0123456789012345@example.com"

	ck1 := make([]byte, 16)
	ik1 := make([]byte, 16)
	ck2 := make([]byte, 16)
	ik2 := make([]byte, 16)
	for i := range ck1 {
		ck1[i] = byte(i)
		ik1[i] = byte(i + 16)
		ck2[i] = byte(i + 32)
		ik2[i] = byte(i + 48)
	}

	km1 := DeriveKeys(identity, ck1, ik1)
	km2 := DeriveKeys(identity, ck2, ik2)

	if bytes.Equal(km1.K_encr, km2.K_encr) {
		t.Error("異なるCK/IKで同一のK_encrが生成された")
	}
	if bytes.Equal(km1.K_aut, km2.K_aut) {
		t.Error("異なるCK/IKで同一のK_autが生成された")
	}
}

func TestDeriveKeys_Deterministic(t *testing.T) {
	identity := "0123456789012345@example.com"
	ck := make([]byte, 16)
	ik := make([]byte, 16)
	for i := range ck {
		ck[i] = byte(i)
		ik[i] = byte(i + 16)
	}

	km1 := DeriveKeys(identity, ck, ik)
	km2 := DeriveKeys(identity, ck, ik)

	if !bytes.Equal(km1.K_encr, km2.K_encr) {
		t.Error("同一入力で異なるK_encrが生成された")
	}
	if !bytes.Equal(km1.K_aut, km2.K_aut) {
		t.Error("同一入力で異なるK_autが生成された")
	}
	if !bytes.Equal(km1.MSK, km2.MSK) {
		t.Error("同一入力で異なるMSKが生成された")
	}
	if !bytes.Equal(km1.EMSK, km2.EMSK) {
		t.Error("同一入力で異なるEMSKが生成された")
	}
}
