package radius

import (
	"crypto/md5"
	"encoding/binary"
	"testing"

	radiuspkg "layeh.com/radius"
)

func TestBuildAccountingResponse(t *testing.T) {
	secret := []byte("testing123")

	request := &radiuspkg.Packet{
		Code:       radiuspkg.CodeAccountingRequest,
		Identifier: 42,
		Secret:     secret,
	}
	// Acct-Status-Type
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, 1)
	request.Add(radiuspkg.Type(AttrTypeAcctStatusType), b)
	request.Add(radiuspkg.Type(AttrTypeAcctSessionID), []byte("sess-123"))

	proxyStates := [][]byte{[]byte("proxy1"), []byte("proxy2")}

	resp := BuildAccountingResponse(request, proxyStates)

	if resp.Code != radiuspkg.CodeAccountingResponse {
		t.Errorf("Code = %d, want %d", resp.Code, radiuspkg.CodeAccountingResponse)
	}
	if resp.Identifier != 42 {
		t.Errorf("Identifier = %d, want 42", resp.Identifier)
	}

	// Proxy-Stateがエコーバックされていることを確認
	var psAttrs [][]byte
	for _, attr := range resp.Attributes {
		if attr.Type == radiuspkg.Type(AttrTypeProxyState) {
			psAttrs = append(psAttrs, attr.Attribute)
		}
	}
	if len(psAttrs) != 2 {
		t.Fatalf("ProxyState count = %d, want 2", len(psAttrs))
	}

	// Encode()前の前提条件: AuthenticatorがRequestAuthenticatorと一致すること
	if resp.Authenticator != request.Authenticator {
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
	copy(encoded[4:20], request.Authenticator[:])
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
