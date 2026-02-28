package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/backend"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-gateway/internal/router"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/httputil"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockBackend はテスト用のバックエンドモック。
type mockBackend struct {
	id       string
	name     string
	response *backend.VectorResponse
	err      error
}

func (m *mockBackend) GetVector(ctx context.Context, req *backend.VectorRequest) (*backend.VectorResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func (m *mockBackend) ID() string   { return m.id }
func (m *mockBackend) Name() string { return m.name }

// mockRegistry はテスト用のRegistry代替。
type mockRegistry struct {
	backend backend.Backend
	err     error
}

func (r *mockRegistry) Get(id string) (backend.Backend, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.backend, nil
}

func (r *mockRegistry) Default() backend.Backend {
	return r.backend
}

// setupHandler はテスト用のVectorHandlerを生成する。
// mockBackendをデフォルトバックエンドとして使うRouterを構築する。
func setupHandler(mock *mockBackend) *VectorHandler {
	cfg := &config.Config{
		InternalURL:     "http://localhost:8080",
		InternalTimeout: 5 * time.Second,
		LogMaskIMSI:     true,
	}
	reg := backend.NewRegistry(cfg)
	r := router.NewRouter(map[string]string{}, reg, true) // passthrough → 常にデフォルト
	return NewVectorHandler(r, cfg)
}

// setupHandlerWithMockServer はモックHTTPサーバーを使ったハンドラーを生成する。
func setupHandlerWithMockServer(srv *httptest.Server) *VectorHandler {
	cfg := &config.Config{
		InternalURL:     srv.URL,
		InternalTimeout: 5 * time.Second,
		LogMaskIMSI:     true,
		Mode:            "passthrough",
	}
	reg := backend.NewRegistry(cfg)
	r := router.NewRouter(map[string]string{}, reg, cfg.IsPassthrough())
	return NewVectorHandler(r, cfg)
}

func TestValidateIMSI(t *testing.T) {
	tests := []struct {
		imsi    string
		wantErr bool
	}{
		{"440101234567890", false},
		{"123456789012345", false},
		{"12345678901234", true},   // 14桁
		{"1234567890123456", true}, // 16桁
		{"44010123456789a", true},  // 非数字
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.imsi, func(t *testing.T) {
			err := validateIMSI(tt.imsi)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIMSI(%q) error = %v, wantErr %v", tt.imsi, err, tt.wantErr)
			}
		})
	}
}

func TestHandleHealth(t *testing.T) {
	cfg := &config.Config{
		InternalURL:     "http://localhost:8080",
		InternalTimeout: 5 * time.Second,
	}
	h := NewVectorHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.HandleHealth(c)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp healthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.Status != "ok" {
		t.Errorf("Status = %q, want %q", resp.Status, "ok")
	}
}

func TestHandleVector_Success(t *testing.T) {
	expected := &backend.VectorResponse{
		RAND: "0102030405060708090a0b0c0d0e0f10",
		AUTN: "1112131415161718191a1b1c1d1e1f20",
		XRES: "2122232425262728",
		CK:   "3132333435363738393a3b3c3d3e3f40",
		IK:   "4142434445464748494a4b4c4d4e4f50",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	h := setupHandlerWithMockServer(srv)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"imsi":"440101234567890"}`
	c.Request, _ = http.NewRequest("POST", "/api/v1/vector", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(TraceIDKey, "test-trace-id")

	h.HandleVector(c)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp backend.VectorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if resp.RAND != expected.RAND {
		t.Errorf("RAND = %q, want %q", resp.RAND, expected.RAND)
	}
}

func TestHandleVector_InvalidJSON(t *testing.T) {
	h := setupHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	c.Request, _ = http.NewRequest("POST", "/api/v1/vector", bytes.NewBufferString("invalid json"))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(TraceIDKey, "test-trace-id")

	h.HandleVector(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleVector_InvalidIMSI(t *testing.T) {
	h := setupHandler(nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"imsi":"12345"}`
	c.Request, _ = http.NewRequest("POST", "/api/v1/vector", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(TraceIDKey, "test-trace-id")

	h.HandleVector(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleVector_Backend4xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(&httputil.ProblemDetail{
			Type:   "about:blank",
			Title:  "Not Found",
			Detail: "subscriber not found",
			Status: 404,
		})
	}))
	defer srv.Close()

	h := setupHandlerWithMockServer(srv)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"imsi":"440101234567890"}`
	c.Request, _ = http.NewRequest("POST", "/api/v1/vector", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(TraceIDKey, "test-trace-id")

	h.HandleVector(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleVector_Backend5xxError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	h := setupHandlerWithMockServer(srv)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"imsi":"440101234567890"}`
	c.Request, _ = http.NewRequest("POST", "/api/v1/vector", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(TraceIDKey, "test-trace-id")

	h.HandleVector(c)

	if w.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestHandleVector_BackendConnectionError(t *testing.T) {
	cfg := &config.Config{
		InternalURL:     "http://127.0.0.1:1",
		InternalTimeout: 1 * time.Second,
		LogMaskIMSI:     true,
		Mode:            "passthrough",
	}
	reg := backend.NewRegistry(cfg)
	r := router.NewRouter(map[string]string{}, reg, cfg.IsPassthrough())
	h := NewVectorHandler(r, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	reqBody := `{"imsi":"440101234567890"}`
	c.Request, _ = http.NewRequest("POST", "/api/v1/vector", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(TraceIDKey, "test-trace-id")

	h.HandleVector(c)

	if w.Code != http.StatusBadGateway {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusBadGateway)
	}
}

func TestHandleVector_RoutingError_BackendNotImplemented(t *testing.T) {
	// PLMNマップを設定し、未登録のバックエンドIDにルーティングさせる
	cfg := &config.Config{
		InternalURL:     "http://localhost:8080",
		InternalTimeout: 5 * time.Second,
		LogMaskIMSI:     true,
		Mode:            "gateway", // gatewayモードでPLMNルーティングを有効化
	}
	reg := backend.NewRegistry(cfg)
	// PLMNマップで "440101" を未登録のバックエンド "99" にルーティング
	plmnMap := map[string]string{"440101": "99"}
	r := router.NewRouter(plmnMap, reg, false) // passthrough=false
	h := NewVectorHandler(r, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// IMSI 440101234567890 は PLMN 440101 にマッチ → バックエンド "99" → 未登録エラー
	reqBody := `{"imsi":"440101234567890"}`
	c.Request, _ = http.NewRequest("POST", "/api/v1/vector", bytes.NewBufferString(reqBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(TraceIDKey, "test-trace-id")

	h.HandleVector(c)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusNotImplemented)
	}
}
