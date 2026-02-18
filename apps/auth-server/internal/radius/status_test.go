package radius

import (
	"crypto/hmac"
	"crypto/md5"
	"testing"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2869"
)

// setValidMessageAuthenticator はリクエストパケットに正しいMessage-Authenticatorを設定する
func setValidMessageAuthenticator(p *radius.Packet, secret []byte) {
	zeroMA := make([]byte, 16)
	_ = rfc2869.MessageAuthenticator_Set(p, zeroMA)
	data, _ := p.MarshalBinary()
	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	_ = rfc2869.MessageAuthenticator_Set(p, mac.Sum(nil))
}

func TestHandleStatusServer_Success(t *testing.T) {
	secret := []byte("status-secret")
	req := radius.New(radius.CodeStatusServer, secret)
	setValidMessageAuthenticator(req, secret)

	resp := HandleStatusServer(req, secret, "192.168.1.1", "trace-001")

	if resp == nil {
		t.Fatal("HandleStatusServer returned nil for valid request")
	}
	if resp.Code != radius.CodeAccessAccept {
		t.Errorf("Code = %d, want %d", resp.Code, radius.CodeAccessAccept)
	}

	// EAP-Messageが含まれないこと
	_, ok := GetEAPMessage(resp)
	if ok {
		t.Error("Status-Server response should not contain EAP-Message")
	}

	// Message-Authenticator存在確認
	_, err := rfc2869.MessageAuthenticator_Lookup(resp)
	if err != nil {
		t.Error("Message-Authenticator not found in response")
	}
}

func TestHandleStatusServer_InvalidAuth(t *testing.T) {
	secret := []byte("status-secret")
	req := radius.New(radius.CodeStatusServer, secret)

	// 不正なMessage-Authenticatorを設定
	invalidMA := make([]byte, 16)
	invalidMA[0] = 0xFF
	_ = rfc2869.MessageAuthenticator_Set(req, invalidMA)

	resp := HandleStatusServer(req, secret, "192.168.1.1", "trace-002")

	if resp != nil {
		t.Error("HandleStatusServer should return nil for invalid MA")
	}
}

func TestHandleStatusServer_WithProxyState(t *testing.T) {
	secret := []byte("status-secret")
	req := radius.New(radius.CodeStatusServer, secret)
	_ = rfc2865.ProxyState_Add(req, []byte("proxy-1"))
	_ = rfc2865.ProxyState_Add(req, []byte("proxy-2"))
	setValidMessageAuthenticator(req, secret)

	resp := HandleStatusServer(req, secret, "192.168.1.1", "trace-003")

	if resp == nil {
		t.Fatal("HandleStatusServer returned nil for valid request")
	}

	// Proxy-Stateがコピーされていること
	states, err := rfc2865.ProxyState_Gets(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 2 {
		t.Fatalf("response has %d ProxyState, want 2", len(states))
	}
	if string(states[0]) != "proxy-1" {
		t.Errorf("ProxyState[0] = %q, want %q", string(states[0]), "proxy-1")
	}
	if string(states[1]) != "proxy-2" {
		t.Errorf("ProxyState[1] = %q, want %q", string(states[1]), "proxy-2")
	}
}

func TestHandleStatusServer_NoMessageAuth(t *testing.T) {
	secret := []byte("status-secret")
	req := radius.New(radius.CodeStatusServer, secret)

	// Message-Authenticator属性なし
	resp := HandleStatusServer(req, secret, "192.168.1.1", "trace-004")

	if resp != nil {
		t.Error("HandleStatusServer should return nil when MA is missing")
	}
}
