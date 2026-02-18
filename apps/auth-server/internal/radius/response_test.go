package radius

import (
	"testing"

	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2868"
	"layeh.com/radius/rfc2869"
)

func newTestRequest(secret []byte) *radius.Packet {
	p := radius.New(radius.CodeAccessRequest, secret)
	_ = rfc2865.UserName_AddString(p, "testuser")
	_ = rfc2869.EAPMessage_Set(p, []byte{0x02, 0x01, 0x00, 0x04})
	return p
}

func TestBuildAccessAccept_Basic(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)

	msk := make([]byte, 64)
	for i := range msk {
		msk[i] = byte(i)
	}

	resp := BuildAccessAccept(req, secret, &AcceptParams{
		EAPMessage:  []byte{0x03, 0x01, 0x00, 0x04}, // EAP-Success
		MSK:         msk,
		SessionID:   "test-session-uuid",
		ProxyStates: &ProxyStates{},
	})

	if resp.Code != radius.CodeAccessAccept {
		t.Errorf("Code = %d, want %d", resp.Code, radius.CodeAccessAccept)
	}

	// EAP-Message確認
	eapMsg, ok := GetEAPMessage(resp)
	if !ok {
		t.Fatal("EAP-Message not found in response")
	}
	if eapMsg[0] != 0x03 {
		t.Errorf("EAP code = %d, want 3 (Success)", eapMsg[0])
	}

	// Class属性確認
	class := rfc2865.Class_Get(resp)
	if string(class) != "test-session-uuid" {
		t.Errorf("Class = %q, want %q", string(class), "test-session-uuid")
	}

	// Message-Authenticator存在確認
	_, err := rfc2869.MessageAuthenticator_Lookup(resp)
	if err != nil {
		t.Error("Message-Authenticator not found in response")
	}
}

func TestBuildAccessAccept_WithVLAN(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)

	msk := make([]byte, 64)

	resp := BuildAccessAccept(req, secret, &AcceptParams{
		EAPMessage:  []byte{0x03, 0x01, 0x00, 0x04},
		MSK:         msk,
		SessionID:   "session-1",
		VlanID:      "100",
		ProxyStates: &ProxyStates{},
	})

	// Tunnel-Type確認（VLAN=13）
	_, tunnelType := rfc2868.TunnelType_Get(resp)
	if tunnelType != 13 {
		t.Errorf("TunnelType = %d, want 13 (VLAN)", tunnelType)
	}

	// Tunnel-Medium-Type確認（IEEE 802=6）
	_, mediumType := rfc2868.TunnelMediumType_Get(resp)
	if mediumType != rfc2868.TunnelMediumType_Value_IEEE802 {
		t.Errorf("TunnelMediumType = %d, want %d (IEEE802)", mediumType, rfc2868.TunnelMediumType_Value_IEEE802)
	}

	// Tunnel-Private-Group-Id確認
	_, groupID := rfc2868.TunnelPrivateGroupID_GetString(resp)
	if groupID != "100" {
		t.Errorf("TunnelPrivateGroupID = %q, want %q", groupID, "100")
	}
}

func TestBuildAccessAccept_WithTimeout(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)

	msk := make([]byte, 64)

	resp := BuildAccessAccept(req, secret, &AcceptParams{
		EAPMessage:     []byte{0x03, 0x01, 0x00, 0x04},
		MSK:            msk,
		SessionID:      "session-1",
		SessionTimeout: 3600,
		ProxyStates:    &ProxyStates{},
	})

	timeout := rfc2865.SessionTimeout_Get(resp)
	if timeout != 3600 {
		t.Errorf("SessionTimeout = %d, want 3600", timeout)
	}
}

func TestBuildAccessAccept_NoTimeout(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)

	msk := make([]byte, 64)

	resp := BuildAccessAccept(req, secret, &AcceptParams{
		EAPMessage:     []byte{0x03, 0x01, 0x00, 0x04},
		MSK:            msk,
		SessionID:      "session-1",
		SessionTimeout: 0,
		ProxyStates:    &ProxyStates{},
	})

	// SessionTimeoutが設定されていないことを確認
	_, err := rfc2865.SessionTimeout_Lookup(resp)
	if err == nil {
		t.Error("SessionTimeout should not be set when value is 0")
	}
}

func TestBuildAccessAccept_ProxyState(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)
	_ = rfc2865.ProxyState_Add(req, []byte("proxy-hop-1"))
	_ = rfc2865.ProxyState_Add(req, []byte("proxy-hop-2"))

	ps := ExtractProxyStates(req)
	msk := make([]byte, 64)

	resp := BuildAccessAccept(req, secret, &AcceptParams{
		EAPMessage:  []byte{0x03, 0x01, 0x00, 0x04},
		MSK:         msk,
		SessionID:   "session-1",
		ProxyStates: ps,
	})

	respStates, err := rfc2865.ProxyState_Gets(resp)
	if err != nil {
		t.Fatal(err)
	}
	if len(respStates) != 2 {
		t.Fatalf("response has %d ProxyState, want 2", len(respStates))
	}
	if string(respStates[0]) != "proxy-hop-1" {
		t.Errorf("ProxyState[0] = %q, want %q", string(respStates[0]), "proxy-hop-1")
	}
}

func TestBuildAccessChallenge(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)

	state := []byte("trace-id-abc123")
	eapChallenge := []byte{0x01, 0x02, 0x00, 0x08, 0x17, 0x01, 0x00, 0x00}

	resp := BuildAccessChallenge(req, secret, &ChallengeParams{
		EAPMessage:  eapChallenge,
		State:       state,
		ProxyStates: &ProxyStates{},
	})

	if resp.Code != radius.CodeAccessChallenge {
		t.Errorf("Code = %d, want %d", resp.Code, radius.CodeAccessChallenge)
	}

	// EAP-Message確認
	eapMsg, ok := GetEAPMessage(resp)
	if !ok {
		t.Fatal("EAP-Message not found")
	}
	if eapMsg[0] != 0x01 {
		t.Errorf("EAP code = %d, want 1 (Request)", eapMsg[0])
	}

	// State確認
	gotState, ok := GetState(resp)
	if !ok {
		t.Fatal("State not found")
	}
	if string(gotState) != string(state) {
		t.Errorf("State = %q, want %q", string(gotState), string(state))
	}

	// Message-Authenticator確認
	_, err := rfc2869.MessageAuthenticator_Lookup(resp)
	if err != nil {
		t.Error("Message-Authenticator not found")
	}
}

func TestBuildAccessChallenge_EAPMessageSplit(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)

	// 253バイト超のEAPメッセージ
	bigEAP := make([]byte, 400)
	for i := range bigEAP {
		bigEAP[i] = byte(i % 256)
	}

	resp := BuildAccessChallenge(req, secret, &ChallengeParams{
		EAPMessage:  bigEAP,
		State:       []byte("state-123"),
		ProxyStates: &ProxyStates{},
	})

	// 結合後のEAPメッセージが元と一致すること
	eapMsg, ok := GetEAPMessage(resp)
	if !ok {
		t.Fatal("EAP-Message not found")
	}
	if len(eapMsg) != 400 {
		t.Errorf("EAP-Message length = %d, want 400", len(eapMsg))
	}
}

func TestBuildAccessReject(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)

	resp := BuildAccessReject(req, secret, &RejectParams{
		EAPMessage:  []byte{0x04, 0x01, 0x00, 0x04}, // EAP-Failure
		ProxyStates: &ProxyStates{},
	})

	if resp.Code != radius.CodeAccessReject {
		t.Errorf("Code = %d, want %d", resp.Code, radius.CodeAccessReject)
	}

	// EAP-Message確認
	eapMsg, ok := GetEAPMessage(resp)
	if !ok {
		t.Fatal("EAP-Message not found")
	}
	if eapMsg[0] != 0x04 {
		t.Errorf("EAP code = %d, want 4 (Failure)", eapMsg[0])
	}
}

func TestBuildAccessReject_HasMessageAuth(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)

	resp := BuildAccessReject(req, secret, &RejectParams{
		EAPMessage:  []byte{0x04, 0x01, 0x00, 0x04},
		ProxyStates: &ProxyStates{},
	})

	// Message-Authenticator存在確認
	ma, err := rfc2869.MessageAuthenticator_Lookup(resp)
	if err != nil {
		t.Fatal("Message-Authenticator not found in Reject response")
	}
	if len(ma) != 16 {
		t.Errorf("Message-Authenticator length = %d, want 16", len(ma))
	}
}

func TestBuildAccessReject_NoEAPMessage(t *testing.T) {
	secret := []byte("test-secret")
	req := newTestRequest(secret)

	resp := BuildAccessReject(req, secret, &RejectParams{
		ProxyStates: &ProxyStates{},
	})

	if resp.Code != radius.CodeAccessReject {
		t.Errorf("Code = %d, want %d", resp.Code, radius.CodeAccessReject)
	}

	// EAP-Messageが設定されていないことを確認
	_, ok := GetEAPMessage(resp)
	if ok {
		t.Error("EAP-Message should not be present when empty")
	}
}
