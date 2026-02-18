package server

import (
	"crypto/hmac"
	"crypto/md5"
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/eap"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/mocks"
	eapaka "github.com/oyaguma3/go-eapaka"
	"go.uber.org/mock/gomock"
	"layeh.com/radius"
	"layeh.com/radius/rfc2869"
)

// mockResponseWriter はradius.ResponseWriterのモック
type mockResponseWriter struct {
	written  []*radius.Packet
	writeErr error
}

func (m *mockResponseWriter) Write(packet *radius.Packet) error {
	m.written = append(m.written, packet)
	return m.writeErr
}

// buildTestAccessRequest はテスト用Access-Requestパケットを構築する
func buildTestAccessRequest(secret []byte, eapMsg []byte) *radius.Packet {
	p := &radius.Packet{
		Code:       radius.CodeAccessRequest,
		Identifier: 1,
		Secret:     secret,
	}
	// EAP-Message設定
	if len(eapMsg) > 0 {
		_ = rfc2869.EAPMessage_Set(p, eapMsg)
	}
	// Message-Authenticator設定（有効な値を生成）
	setValidMessageAuthenticator(p, secret)
	return p
}

// setValidMessageAuthenticator はパケットに有効なMessage-Authenticatorを設定する
func setValidMessageAuthenticator(p *radius.Packet, secret []byte) {
	_ = rfc2869.MessageAuthenticator_Set(p, make([]byte, 16))
	data, err := p.MarshalBinary()
	if err != nil {
		return
	}
	mac := hmac.New(md5.New, secret)
	mac.Write(data)
	_ = rfc2869.MessageAuthenticator_Set(p, mac.Sum(nil))
}

// buildTestEAPIdentity はEAP-Response/Identityパケットを構築する
func buildTestEAPIdentity() []byte {
	pkt := &eapaka.Packet{
		Code:       eapaka.CodeResponse,
		Identifier: 1,
		Type:       eapaka.TypeAKA,
		Subtype:    eapaka.SubtypeIdentity,
	}
	data, _ := pkt.Marshal()
	return data
}

func TestHandler_AccessRequest_Accept(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)

	msk := make([]byte, 64)
	mockEngine.EXPECT().Process(gomock.Any(), gomock.Any()).
		Return(&eap.Result{
			Action:         eap.ActionAccept,
			EAPMessage:     []byte{3, 2, 0, 4}, // EAP-Success
			MSK:            msk,
			SessionID:      "test-session",
			VlanID:         "100",
			SessionTimeout: 3600,
		}, nil)

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	eapMsg := buildTestEAPIdentity()
	p := buildTestAccessRequest(secret, eapMsg)

	rw := &mockResponseWriter{}
	req := &radius.Request{
		Packet: p,
	}

	handler.ServeRADIUS(rw, req)

	if len(rw.written) != 1 {
		t.Fatalf("written packets: got %d, want 1", len(rw.written))
	}
	if rw.written[0].Code != radius.CodeAccessAccept {
		t.Errorf("Code: got %v, want %v", rw.written[0].Code, radius.CodeAccessAccept)
	}
}

func TestHandler_AccessRequest_Challenge(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)
	mockEngine.EXPECT().Process(gomock.Any(), gomock.Any()).
		Return(&eap.Result{
			Action:     eap.ActionChallenge,
			EAPMessage: []byte{1, 2, 0, 8, 23, 5, 0, 0},
			State:      []byte("trace-id"),
		}, nil)

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	eapMsg := buildTestEAPIdentity()
	p := buildTestAccessRequest(secret, eapMsg)

	rw := &mockResponseWriter{}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	if len(rw.written) != 1 {
		t.Fatalf("written packets: got %d, want 1", len(rw.written))
	}
	if rw.written[0].Code != radius.CodeAccessChallenge {
		t.Errorf("Code: got %v, want %v", rw.written[0].Code, radius.CodeAccessChallenge)
	}
}

func TestHandler_AccessRequest_Reject(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)
	mockEngine.EXPECT().Process(gomock.Any(), gomock.Any()).
		Return(&eap.Result{
			Action:     eap.ActionReject,
			EAPMessage: []byte{4, 2, 0, 4}, // EAP-Failure
		}, nil)

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	eapMsg := buildTestEAPIdentity()
	p := buildTestAccessRequest(secret, eapMsg)

	rw := &mockResponseWriter{}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	if len(rw.written) != 1 {
		t.Fatalf("written packets: got %d, want 1", len(rw.written))
	}
	if rw.written[0].Code != radius.CodeAccessReject {
		t.Errorf("Code: got %v, want %v", rw.written[0].Code, radius.CodeAccessReject)
	}
}

func TestHandler_AccessRequest_Drop(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)
	mockEngine.EXPECT().Process(gomock.Any(), gomock.Any()).
		Return(&eap.Result{
			Action: eap.ActionDrop,
		}, nil)

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	eapMsg := buildTestEAPIdentity()
	p := buildTestAccessRequest(secret, eapMsg)

	rw := &mockResponseWriter{}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	if len(rw.written) != 0 {
		t.Errorf("written packets: got %d, want 0 (drop)", len(rw.written))
	}
}

func TestHandler_AccessRequest_NoMA(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)
	// Process呼び出しは期待しない

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	p := &radius.Packet{
		Code:       radius.CodeAccessRequest,
		Identifier: 1,
		Secret:     secret,
	}
	// Message-Authenticatorなし → 検証失敗

	rw := &mockResponseWriter{}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	if len(rw.written) != 0 {
		t.Errorf("written packets: got %d, want 0 (MA verification failed)", len(rw.written))
	}
}

func TestHandler_StatusServer(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	p := &radius.Packet{
		Code:       radius.CodeStatusServer,
		Identifier: 1,
		Secret:     secret,
	}
	// 有効なMessage-Authenticatorを設定
	setValidMessageAuthenticator(p, secret)

	rw := &mockResponseWriter{}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	if len(rw.written) != 1 {
		t.Fatalf("written packets: got %d, want 1", len(rw.written))
	}
	if rw.written[0].Code != radius.CodeAccessAccept {
		t.Errorf("Code: got %v, want %v", rw.written[0].Code, radius.CodeAccessAccept)
	}
}

func TestHandler_StatusServer_InvalidMA(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	p := &radius.Packet{
		Code:       radius.CodeStatusServer,
		Identifier: 1,
		Secret:     secret,
	}
	// 不正なMessage-Authenticatorを設定
	invalidMA := make([]byte, 16)
	invalidMA[0] = 0xFF
	_ = rfc2869.MessageAuthenticator_Set(p, invalidMA)

	rw := &mockResponseWriter{}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	// Message-Authenticator検証失敗 → 無応答
	if len(rw.written) != 0 {
		t.Errorf("written packets: got %d, want 0 (MA verification failed)", len(rw.written))
	}
}

func TestHandler_StatusServer_NoMA(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	p := &radius.Packet{
		Code:       radius.CodeStatusServer,
		Identifier: 1,
		Secret:     secret,
	}
	// Message-Authenticatorなし

	rw := &mockResponseWriter{}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	// Message-Authenticatorなし → 無応答
	if len(rw.written) != 0 {
		t.Errorf("written packets: got %d, want 0 (no MA)", len(rw.written))
	}
}

func TestHandler_UnknownCode(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)

	handler := NewHandler(mockEngine)

	p := &radius.Packet{
		Code:       radius.CodeAccountingRequest,
		Identifier: 1,
		Secret:     []byte("test-secret"),
	}

	rw := &mockResponseWriter{}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	if len(rw.written) != 0 {
		t.Errorf("written packets: got %d, want 0", len(rw.written))
	}
}

func TestHandler_AccessRequest_EngineError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)
	mockEngine.EXPECT().Process(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("engine error"))

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	eapMsg := buildTestEAPIdentity()
	p := buildTestAccessRequest(secret, eapMsg)

	rw := &mockResponseWriter{}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	// エンジンエラー → 応答なし
	if len(rw.written) != 0 {
		t.Errorf("written packets: got %d, want 0 (engine error)", len(rw.written))
	}
}

func TestHandler_AccessRequest_WriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)
	msk := make([]byte, 64)
	mockEngine.EXPECT().Process(gomock.Any(), gomock.Any()).
		Return(&eap.Result{
			Action:         eap.ActionAccept,
			EAPMessage:     []byte{3, 2, 0, 4},
			MSK:            msk,
			SessionID:      "test-session",
			VlanID:         "100",
			SessionTimeout: 3600,
		}, nil)

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	eapMsg := buildTestEAPIdentity()
	p := buildTestAccessRequest(secret, eapMsg)

	rw := &mockResponseWriter{writeErr: errors.New("write error")}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	// Write自体は呼ばれるが、エラーログのみ
	if len(rw.written) != 1 {
		t.Fatalf("written packets: got %d, want 1", len(rw.written))
	}
}

func TestHandler_StatusServer_WriteError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockEngine := mocks.NewMockEAPProcessor(ctrl)

	handler := NewHandler(mockEngine)

	secret := []byte("test-secret")
	p := &radius.Packet{
		Code:       radius.CodeStatusServer,
		Identifier: 1,
		Secret:     secret,
	}
	// 有効なMessage-Authenticatorを設定
	setValidMessageAuthenticator(p, secret)

	rw := &mockResponseWriter{writeErr: errors.New("write error")}
	req := &radius.Request{Packet: p}

	handler.ServeRADIUS(rw, req)

	// Write自体は呼ばれるが、エラーログのみ
	if len(rw.written) != 1 {
		t.Fatalf("written packets: got %d, want 1", len(rw.written))
	}
}
