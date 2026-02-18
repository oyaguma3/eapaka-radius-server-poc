package vector

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/config"
)

// テスト用の認証ベクター（固定値）
var testVector = vectorResponseJSON{
	RAND: "f4b38a1c2d3e4f5a6b7c8d9e0f1a2b3c",
	AUTN: "2b9e10a3b4c5d6e7f8091a2b3c4d5e6f",
	XRES: "d8a1b2c3d4e5f6a7",
	CK:   "91e3a4b5c6d7e8f90a1b2c3d4e5f6a7b",
	IK:   "c42fa1b2c3d4e5f6a7b8c9d0e1f2a3b4",
}

func newTestConfig(url string) *config.Config {
	return &config.Config{
		VectorAPIURL: url,
		NetworkName:  "WLAN",
		RedisHost:    "localhost",
		RedisPort:    "6379",
		RedisPass:    "",
	}
}

func ctxWithTrace() context.Context {
	return WithTraceID(context.Background(), "test-trace-id-001")
}

func TestGetVectorSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// リクエスト検証
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/vector" {
			t.Errorf("expected /api/v1/vector, got %s", r.URL.Path)
		}
		if r.Header.Get(HeaderTraceID) == "" {
			t.Error("expected X-Trace-ID header")
		}
		if r.Header.Get(HeaderContentType) != ContentTypeJSON {
			t.Errorf("expected Content-Type %s, got %s", ContentTypeJSON, r.Header.Get(HeaderContentType))
		}

		// リクエストボディ検証
		var req VectorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.IMSI != "440101234567890" {
			t.Errorf("expected IMSI 440101234567890, got %s", req.IMSI)
		}

		w.Header().Set("Content-Type", ContentTypeJSON)
		json.NewEncoder(w).Encode(testVector)
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))
	resp, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
	if err != nil {
		t.Fatalf("GetVector failed: %v", err)
	}

	// レスポンス検証
	expectedRAND, _ := hex.DecodeString(testVector.RAND)
	if !bytesEqual(resp.RAND, expectedRAND) {
		t.Errorf("RAND mismatch: got %x, want %x", resp.RAND, expectedRAND)
	}
	expectedAUTN, _ := hex.DecodeString(testVector.AUTN)
	if !bytesEqual(resp.AUTN, expectedAUTN) {
		t.Errorf("AUTN mismatch")
	}
	if len(resp.XRES) == 0 {
		t.Error("XRES is empty")
	}
	if len(resp.CK) != 16 {
		t.Errorf("CK length = %d, want 16", len(resp.CK))
	}
	if len(resp.IK) != 16 {
		t.Errorf("IK length = %d, want 16", len(resp.IK))
	}
}

func TestGetVectorWithResync(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req VectorRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if req.ResyncInfo == nil {
			t.Fatal("expected resync_info to be present")
		}
		if req.ResyncInfo.RAND == "" || req.ResyncInfo.AUTS == "" {
			t.Error("resync_info fields should not be empty")
		}

		w.Header().Set("Content-Type", ContentTypeJSON)
		json.NewEncoder(w).Encode(testVector)
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))
	resp, err := client.GetVector(ctxWithTrace(), &VectorRequest{
		IMSI: "440101234567890",
		ResyncInfo: &ResyncInfo{
			RAND: "aabbccdd11223344aabbccdd11223344",
			AUTS: "112233445566778899aabbccddee",
		},
	})
	if err != nil {
		t.Fatalf("GetVector with resync failed: %v", err)
	}
	if resp == nil {
		t.Fatal("response is nil")
	}
}

func TestGetVectorNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentTypeJSON)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(ProblemDetails{
			Type:   "about:blank",
			Title:  "Not Found",
			Detail: "subscriber not found",
			Status: 404,
		})
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))
	_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "999999999999999"})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if !apiErr.IsNotFound() {
		t.Errorf("expected IsNotFound() = true, got false (status=%d)", apiErr.StatusCode)
	}
}

func TestGetVectorBadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentTypeJSON)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ProblemDetails{
			Type:   "about:blank",
			Title:  "Bad Request",
			Detail: "invalid IMSI format",
			Status: 400,
		})
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))
	_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "invalid"})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if !apiErr.IsBadRequest() {
		t.Errorf("expected IsBadRequest() = true")
	}
}

func TestGetVectorServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"type":"about:blank","title":"Internal Server Error","detail":"db error","status":500}`))
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))
	_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if !apiErr.IsServerError() {
		t.Errorf("expected IsServerError() = true")
	}
}

func TestGetVectorCircuitBreakerOpen(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"type":"about:blank","title":"Error","detail":"error","status":500}`))
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))

	// CBFailureThreshold(5)回連続失敗させてCircuit BreakerをOpenにする
	for i := 0; i < config.CBFailureThreshold; i++ {
		client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
	}

	// 次のリクエストはCircuit Breaker Openで即座に失敗するはず
	_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
	if err == nil {
		t.Fatal("expected error when circuit breaker is open")
	}
	if !errors.Is(err, ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got: %v", err)
	}
}

func TestGetVectorConnectionError(t *testing.T) {
	// 存在しないサーバーへ接続
	client := NewClient(newTestConfig("http://127.0.0.1:59999"))
	_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
	if err == nil {
		t.Fatal("expected connection error")
	}

	var connErr *ConnectionError
	if !errors.As(err, &connErr) {
		t.Fatalf("expected ConnectionError, got %T: %v", err, err)
	}
}

func TestGetVectorTraceIDMissing(t *testing.T) {
	client := NewClient(newTestConfig("http://localhost:8080"))
	// Trace IDなしのコンテキスト
	_, err := client.GetVector(context.Background(), &VectorRequest{IMSI: "440101234567890"})
	if err == nil {
		t.Fatal("expected error for missing trace ID")
	}
	if !errors.Is(err, ErrTraceIDMissing) {
		t.Errorf("expected ErrTraceIDMissing, got: %v", err)
	}
}

func TestGetVectorInvalidResponseJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentTypeJSON)
		w.Write([]byte(`not json`))
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))
	_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("expected ErrInvalidResponse, got: %v", err)
	}
}

func TestGetVectorInvalidHex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", ContentTypeJSON)
		json.NewEncoder(w).Encode(vectorResponseJSON{
			RAND: "not-valid-hex",
			AUTN: "2b9e10a3b4c5d6e7f8091a2b3c4d5e6f",
			XRES: "d8a1b2c3d4e5f6a7",
			CK:   "91e3a4b5c6d7e8f90a1b2c3d4e5f6a7b",
			IK:   "c42fa1b2c3d4e5f6a7b8c9d0e1f2a3b4",
		})
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))
	_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
	if err == nil {
		t.Fatal("expected error for invalid hex in response")
	}
	if !errors.Is(err, ErrInvalidResponse) {
		t.Errorf("expected ErrInvalidResponse, got: %v", err)
	}
}

func TestGetVector501NotCountedByCB(t *testing.T) {
	// 501はCB対象外であることを確認
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(ProblemDetails{
			Type:   "about:blank",
			Title:  "Not Implemented",
			Detail: "not implemented",
			Status: 501,
		})
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))

	// CBFailureThreshold回呼んでもCBがOpenにならないことを確認
	for i := 0; i < config.CBFailureThreshold+1; i++ {
		_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
		if err == nil {
			t.Fatal("expected error for 501 response")
		}
		// ErrCircuitOpenにはならないはず
		if errors.Is(err, ErrCircuitOpen) {
			t.Fatalf("501 should not trigger circuit breaker open (iteration %d)", i)
		}
	}
}

func TestWithTraceID(t *testing.T) {
	ctx := WithTraceID(context.Background(), "abc-123")
	val, ok := ctx.Value(traceIDKey{}).(string)
	if !ok {
		t.Fatal("trace ID not found in context")
	}
	if val != "abc-123" {
		t.Errorf("trace ID = %q, want %q", val, "abc-123")
	}
}

// TestConnectionError_Methods はConnectionErrorのError()とUnwrap()をカバーする
func TestConnectionError_Methods(t *testing.T) {
	cause := errors.New("dial tcp: connection refused")
	connErr := &ConnectionError{Cause: cause}

	// Error()のフォーマット検証
	expected := "connection error: dial tcp: connection refused"
	if connErr.Error() != expected {
		t.Errorf("Error() = %q, want %q", connErr.Error(), expected)
	}

	// Unwrap()でCauseが返ることを検証
	if connErr.Unwrap() != cause {
		t.Errorf("Unwrap() returned unexpected error: %v", connErr.Unwrap())
	}

	// errors.Is経由の一致検証
	if !errors.Is(connErr, cause) {
		t.Error("expected errors.Is(connErr, cause) = true")
	}
}

// TestAPIError_ErrorWithoutDetails はDetails=nilの場合のAPIError.Error()をカバーする
func TestAPIError_ErrorWithoutDetails(t *testing.T) {
	apiErr := &APIError{
		StatusCode: 500,
		Message:    "internal error",
		Details:    nil,
	}

	expected := "vector api error: 500 internal error"
	if apiErr.Error() != expected {
		t.Errorf("Error() = %q, want %q", apiErr.Error(), expected)
	}
}

// TestGetVectorInvalidHex_AllFields はAUTN/XRES/CK/IK各フィールドのhexデコード失敗をカバーする
func TestGetVectorInvalidHex_AllFields(t *testing.T) {
	validHex := struct {
		RAND string
		AUTN string
		XRES string
		CK   string
		IK   string
	}{
		RAND: "f4b38a1c2d3e4f5a6b7c8d9e0f1a2b3c",
		AUTN: "2b9e10a3b4c5d6e7f8091a2b3c4d5e6f",
		XRES: "d8a1b2c3d4e5f6a7",
		CK:   "91e3a4b5c6d7e8f90a1b2c3d4e5f6a7b",
		IK:   "c42fa1b2c3d4e5f6a7b8c9d0e1f2a3b4",
	}

	tests := []struct {
		name string
		resp vectorResponseJSON
	}{
		{
			name: "invalid_autn",
			resp: vectorResponseJSON{
				RAND: validHex.RAND, AUTN: "ZZZZ", XRES: validHex.XRES, CK: validHex.CK, IK: validHex.IK,
			},
		},
		{
			name: "invalid_xres",
			resp: vectorResponseJSON{
				RAND: validHex.RAND, AUTN: validHex.AUTN, XRES: "ZZZZ", CK: validHex.CK, IK: validHex.IK,
			},
		},
		{
			name: "invalid_ck",
			resp: vectorResponseJSON{
				RAND: validHex.RAND, AUTN: validHex.AUTN, XRES: validHex.XRES, CK: "ZZZZ", IK: validHex.IK,
			},
		},
		{
			name: "invalid_ik",
			resp: vectorResponseJSON{
				RAND: validHex.RAND, AUTN: validHex.AUTN, XRES: validHex.XRES, CK: validHex.CK, IK: "ZZZZ",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", ContentTypeJSON)
				json.NewEncoder(w).Encode(tt.resp)
			}))
			defer server.Close()

			client := NewClient(newTestConfig(server.URL))
			_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
			if err == nil {
				t.Fatal("expected error for invalid hex")
			}
			if !errors.Is(err, ErrInvalidResponse) {
				t.Errorf("expected ErrInvalidResponse, got: %v", err)
			}
		})
	}
}

// TestGetVectorServerError_InvalidJSON はparseAPIErrorのJSONパース失敗フォールバックをカバーする
func TestGetVectorServerError_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("not json at all"))
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))
	_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
	if err == nil {
		t.Fatal("expected error for 500 with invalid JSON body")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.Details != nil {
		t.Errorf("expected Details to be nil, got %+v", apiErr.Details)
	}
	if apiErr.Message != "not json at all" {
		t.Errorf("expected Message = %q, got %q", "not json at all", apiErr.Message)
	}
}

// TestGetVectorServerError_EmptyTitle はparseAPIErrorのTitle空フォールバックをカバーする
func TestGetVectorServerError_EmptyTitle(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"type":"about:blank","title":"","detail":"some detail","status":500}`))
	}))
	defer server.Close()

	client := NewClient(newTestConfig(server.URL))
	_, err := client.GetVector(ctxWithTrace(), &VectorRequest{IMSI: "440101234567890"})
	if err == nil {
		t.Fatal("expected error for 500 with empty title")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	// Title空の場合、parseAPIErrorはフォールバックしてDetails=nilになる
	if apiErr.Details != nil {
		t.Errorf("expected Details to be nil for empty title, got %+v", apiErr.Details)
	}
}

// bytesEqual はバイトスライスの比較ヘルパー
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
