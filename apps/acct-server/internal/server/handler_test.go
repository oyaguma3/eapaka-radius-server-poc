package server

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"net"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
	radiuspkg "layeh.com/radius"
	"layeh.com/radius/rfc2869"
)

// mockProcessor はテスト用のAccountingProcessor実装
type mockProcessor struct {
	startCalled   bool
	interimCalled bool
	stopCalled    bool
	lastTraceID   string
	returnErr     error
}

func (m *mockProcessor) ProcessStart(_ context.Context, _ *radius.AccountingAttributes, _, traceID string) error {
	m.startCalled = true
	m.lastTraceID = traceID
	return m.returnErr
}

func (m *mockProcessor) ProcessInterim(_ context.Context, _ *radius.AccountingAttributes, _, traceID string) error {
	m.interimCalled = true
	m.lastTraceID = traceID
	return m.returnErr
}

func (m *mockProcessor) ProcessStop(_ context.Context, _ *radius.AccountingAttributes, _, traceID string) error {
	m.stopCalled = true
	m.lastTraceID = traceID
	return m.returnErr
}

// mockResponseWriter はテスト用のResponseWriter実装
type mockResponseWriter struct {
	written  *radiuspkg.Packet
	writeErr error
}

func (m *mockResponseWriter) Write(packet *radiuspkg.Packet) error {
	m.written = packet
	return m.writeErr
}

// mockAddr はテスト用のnet.Addr実装
type mockAddr struct {
	addr string
}

func (m *mockAddr) Network() string { return "udp" }
func (m *mockAddr) String() string  { return m.addr }

func TestNewHandler(t *testing.T) {
	proc := &mockProcessor{}
	h := NewHandler(proc)
	if h == nil {
		t.Fatal("NewHandler returned nil")
	}
	if h.processor != proc {
		t.Error("processor not set correctly")
	}
}

// createAccountingRequest はテスト用のAccounting-Requestを作成する
func createAccountingRequest(t *testing.T, secret []byte, statusType uint32) *radiuspkg.Request {
	t.Helper()
	packet := &radiuspkg.Packet{
		Code:       radiuspkg.CodeAccountingRequest,
		Identifier: 1,
		Secret:     secret,
	}

	// Acct-Status-Type
	statusData := make([]byte, 4)
	binary.BigEndian.PutUint32(statusData, statusType)
	packet.Add(radiuspkg.Type(radius.AttrTypeAcctStatusType), statusData)

	// Acct-Session-Id
	packet.Add(radiuspkg.Type(radius.AttrTypeAcctSessionID), []byte("test-session-id"))

	// 正しいAuthenticatorを計算
	data, err := packet.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary failed: %v", err)
	}
	copy(data[4:20], make([]byte, 16))
	h := md5.New()
	h.Write(data)
	h.Write(secret)
	copy(packet.Authenticator[:], h.Sum(nil))

	return &radiuspkg.Request{
		Packet:     packet,
		RemoteAddr: &mockAddr{addr: "192.168.1.1:12345"},
	}
}

// createStatusServerRequest はテスト用のStatus-Serverリクエストを作成する
func createStatusServerRequest(t *testing.T, secret []byte, withValidMA bool) *radiuspkg.Request {
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

	return &radiuspkg.Request{
		Packet:     packet,
		RemoteAddr: &mockAddr{addr: "192.168.1.1:12345"},
	}
}

func TestServeRADIUS_AccountingRequest_Start(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}
	r := createAccountingRequest(t, secret, radius.AcctStatusTypeStart)

	h.ServeRADIUS(w, r)

	if !proc.startCalled {
		t.Error("ProcessStart should be called")
	}
	if w.written == nil {
		t.Error("Response should be written")
	}
	if w.written != nil && w.written.Code != radiuspkg.CodeAccountingResponse {
		t.Errorf("Response Code = %v, want %v", w.written.Code, radiuspkg.CodeAccountingResponse)
	}
}

func TestServeRADIUS_AccountingRequest_Stop(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}
	r := createAccountingRequest(t, secret, radius.AcctStatusTypeStop)

	h.ServeRADIUS(w, r)

	if !proc.stopCalled {
		t.Error("ProcessStop should be called")
	}
	if w.written == nil {
		t.Error("Response should be written")
	}
}

func TestServeRADIUS_AccountingRequest_Interim(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}
	r := createAccountingRequest(t, secret, radius.AcctStatusTypeInterim)

	h.ServeRADIUS(w, r)

	if !proc.interimCalled {
		t.Error("ProcessInterim should be called")
	}
	if w.written == nil {
		t.Error("Response should be written")
	}
}

func TestServeRADIUS_InvalidAuthenticator(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}
	r := createAccountingRequest(t, secret, radius.AcctStatusTypeStart)

	// Authenticatorを破損させる
	r.Packet.Authenticator[0] ^= 0xFF

	h.ServeRADIUS(w, r)

	if proc.startCalled {
		t.Error("ProcessStart should not be called for invalid authenticator")
	}
	if w.written != nil {
		t.Error("Response should not be written for invalid authenticator")
	}
}

func TestServeRADIUS_MissingAttributes(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}

	// 属性なしのパケット
	packet := &radiuspkg.Packet{
		Code:       radiuspkg.CodeAccountingRequest,
		Identifier: 1,
		Secret:     secret,
	}

	// 正しいAuthenticatorを計算
	data, _ := packet.MarshalBinary()
	copy(data[4:20], make([]byte, 16))
	md5Hash := md5.New()
	md5Hash.Write(data)
	md5Hash.Write(secret)
	copy(packet.Authenticator[:], md5Hash.Sum(nil))

	r := &radiuspkg.Request{
		Packet:     packet,
		RemoteAddr: &mockAddr{addr: "192.168.1.1:12345"},
	}

	h.ServeRADIUS(w, r)

	if proc.startCalled || proc.interimCalled || proc.stopCalled {
		t.Error("No processor method should be called for missing attributes")
	}
	if w.written != nil {
		t.Error("Response should not be written for missing attributes")
	}
}

func TestServeRADIUS_UnknownStatusType(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}
	r := createAccountingRequest(t, secret, 7) // 未知のStatusType

	h.ServeRADIUS(w, r)

	if proc.startCalled || proc.interimCalled || proc.stopCalled {
		t.Error("No processor method should be called for unknown status type")
	}
	if w.written != nil {
		t.Error("Response should not be written for unknown status type")
	}
}

func TestServeRADIUS_UnknownCode(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}

	packet := &radiuspkg.Packet{
		Code:       radiuspkg.Code(99), // 未知のCode
		Identifier: 1,
		Secret:     secret,
	}

	r := &radiuspkg.Request{
		Packet:     packet,
		RemoteAddr: &mockAddr{addr: "192.168.1.1:12345"},
	}

	h.ServeRADIUS(w, r)

	if proc.startCalled || proc.interimCalled || proc.stopCalled {
		t.Error("No processor method should be called for unknown code")
	}
	if w.written != nil {
		t.Error("Response should not be written for unknown code")
	}
}

func TestServeRADIUS_StatusServer(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}
	r := createStatusServerRequest(t, secret, true)

	h.ServeRADIUS(w, r)

	if w.written == nil {
		t.Error("Response should be written for Status-Server")
	}
	if w.written != nil && w.written.Code != radiuspkg.CodeAccountingResponse {
		t.Errorf("Response Code = %v, want %v", w.written.Code, radiuspkg.CodeAccountingResponse)
	}
}

func TestServeRADIUS_StatusServer_InvalidMA(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}
	r := createStatusServerRequest(t, secret, false) // MAなし

	h.ServeRADIUS(w, r)

	if w.written != nil {
		t.Error("Response should not be written for invalid MA")
	}
}

func TestServeRADIUS_ProcessorError(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{returnErr: errors.New("test error")}
	h := NewHandler(proc)
	w := &mockResponseWriter{}
	r := createAccountingRequest(t, secret, radius.AcctStatusTypeStart)

	h.ServeRADIUS(w, r)

	// エラーがあっても応答は返す
	if w.written == nil {
		t.Error("Response should be written even on processor error")
	}
}

func TestServeRADIUS_WriteError(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{writeErr: errors.New("write error")}
	r := createAccountingRequest(t, secret, radius.AcctStatusTypeStart)

	// パニックしないことを確認
	h.ServeRADIUS(w, r)

	if !proc.startCalled {
		t.Error("ProcessStart should be called")
	}
}

func TestServeRADIUS_StatusServer_WriteError(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{writeErr: errors.New("write error")}
	r := createStatusServerRequest(t, secret, true)

	// パニックしないことを確認
	h.ServeRADIUS(w, r)
}

func TestHandlerWithUDPAddress(t *testing.T) {
	secret := []byte("testing123")
	proc := &mockProcessor{}
	h := NewHandler(proc)
	w := &mockResponseWriter{}
	r := createAccountingRequest(t, secret, radius.AcctStatusTypeStart)

	// net.UDPAddrを使用
	r.RemoteAddr = &net.UDPAddr{
		IP:   net.ParseIP("10.0.0.1"),
		Port: 12345,
	}

	h.ServeRADIUS(w, r)

	if !proc.startCalled {
		t.Error("ProcessStart should be called")
	}
}
