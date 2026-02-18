package radius

import (
	"crypto/hmac"
	"crypto/md5"
	"testing"

	radiuspkg "layeh.com/radius"
	"layeh.com/radius/rfc2869"
)

// createStatusServerRequest はテスト用のStatus-Serverリクエストを作成する
func createStatusServerRequest(t *testing.T, secret []byte, withValidMA bool) *radiuspkg.Packet {
	t.Helper()
	packet := &radiuspkg.Packet{
		Code:       radiuspkg.CodeStatusServer,
		Identifier: 1,
		Secret:     secret,
	}

	if withValidMA {
		// 正しいMAを設定
		zeroMA := make([]byte, 16)
		_ = rfc2869.MessageAuthenticator_Set(packet, zeroMA)

		data, err := packet.MarshalBinary()
		if err != nil {
			t.Fatalf("MarshalBinary failed: %v", err)
		}

		mac := hmac.New(md5.New, secret)
		mac.Write(data)
		_ = rfc2869.MessageAuthenticator_Set(packet, mac.Sum(nil))
	}

	return packet
}

func TestHandleStatusServer_Success(t *testing.T) {
	secret := []byte("testing123")
	packet := createStatusServerRequest(t, secret, true)

	resp := HandleStatusServer(packet, secret, "192.168.1.1", "trace-001")

	if resp == nil {
		t.Fatal("HandleStatusServer should return a response for valid MA")
	}

	if resp.Code != radiuspkg.CodeAccountingResponse {
		t.Errorf("Response Code = %v, want %v", resp.Code, radiuspkg.CodeAccountingResponse)
	}

	// 応答にMessage-Authenticatorが設定されていることを確認
	ma, err := rfc2869.MessageAuthenticator_Lookup(resp)
	if err != nil {
		t.Error("Response should have Message-Authenticator")
	}
	if len(ma) != 16 {
		t.Errorf("MA length = %d, want 16", len(ma))
	}
}

func TestHandleStatusServer_InvalidMA(t *testing.T) {
	secret := []byte("testing123")
	packet := createStatusServerRequest(t, secret, true)

	// MAを改ざん
	ma, _ := rfc2869.MessageAuthenticator_Lookup(packet)
	ma[0] ^= 0xFF
	_ = rfc2869.MessageAuthenticator_Set(packet, ma)

	resp := HandleStatusServer(packet, secret, "192.168.1.1", "trace-001")

	if resp != nil {
		t.Error("HandleStatusServer should return nil for invalid MA")
	}
}

func TestHandleStatusServer_MissingMA(t *testing.T) {
	secret := []byte("testing123")
	packet := createStatusServerRequest(t, secret, false) // MAなし

	resp := HandleStatusServer(packet, secret, "192.168.1.1", "trace-001")

	if resp != nil {
		t.Error("HandleStatusServer should return nil for missing MA")
	}
}

func TestHandleStatusServer_WrongSecret(t *testing.T) {
	secret := []byte("testing123")
	wrongSecret := []byte("wrong")
	packet := createStatusServerRequest(t, secret, true)

	resp := HandleStatusServer(packet, wrongSecret, "192.168.1.1", "trace-001")

	if resp != nil {
		t.Error("HandleStatusServer should return nil for wrong secret")
	}
}

func TestHandleStatusServer_ResponseAuthenticator(t *testing.T) {
	secret := []byte("testing123")
	packet := createStatusServerRequest(t, secret, true)

	resp := HandleStatusServer(packet, secret, "192.168.1.1", "trace-001")

	if resp == nil {
		t.Fatal("HandleStatusServer should return a response")
	}

	// Encode()前の前提条件: AuthenticatorがRequestAuthenticatorと一致すること
	if resp.Authenticator != packet.Authenticator {
		t.Error("Encode()前のAuthenticatorはRequestAuthenticatorと一致する必要がある")
	}

	// Encode()経由でResponse Authenticatorが正しく計算されることを検証
	encoded, err := resp.Encode()
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	// エンコード結果からResponse Authenticatorを取得
	var respAuth [16]byte
	copy(respAuth[:], encoded[4:20])

	// RFC 2866に基づく期待値を計算: MD5(Code+ID+Length+RequestAuth+Attrs+Secret)
	copy(encoded[4:20], packet.Authenticator[:])
	h := md5.New()
	h.Write(encoded)
	h.Write(secret)
	expected := h.Sum(nil)

	for i := 0; i < 16; i++ {
		if respAuth[i] != expected[i] {
			t.Errorf("Response Authenticator mismatch at byte %d", i)
			break
		}
	}
}
