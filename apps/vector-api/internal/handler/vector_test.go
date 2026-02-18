package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/vector-api/internal/usecase"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// mockVectorUseCase はテスト用のモック
type mockVectorUseCase struct {
	response *dto.VectorResponse
	err      error
}

func (m *mockVectorUseCase) GenerateVector(ctx context.Context, req *dto.VectorRequest) (*dto.VectorResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
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

func TestMaskIMSI(t *testing.T) {
	tests := []struct {
		name     string
		imsi     string
		maskIMSI bool
		want     string
	}{
		{"mask enabled", "440101234567890", true, "44010********90"},
		{"mask disabled", "440101234567890", false, "440101234567890"},
		{"short IMSI", "1234567", true, "1234567"},
		{"empty", "", true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{LogMaskIMSI: tt.maskIMSI}
			h := &VectorHandler{cfg: cfg}
			got := h.maskIMSI(tt.imsi)
			if got != tt.want {
				t.Errorf("maskIMSI(%q) = %q, want %q", tt.imsi, got, tt.want)
			}
		})
	}
}

func TestHandleHealth(t *testing.T) {
	cfg := &config.Config{}
	h := NewVectorHandler(nil, cfg)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	h.HandleHealth(c)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp dto.HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Status = %q, want %q", resp.Status, "ok")
	}
}

func TestHandleVector(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		mockUC := &mockVectorUseCase{
			response: &dto.VectorResponse{
				RAND: "0102030405060708090a0b0c0d0e0f10",
				AUTN: "1112131415161718191a1b1c1d1e1f20",
				XRES: "2122232425262728",
				CK:   "3132333435363738393a3b3c3d3e3f40",
				IK:   "4142434445464748494a4b4c4d4e4f50",
			},
		}
		cfg := &config.Config{LogMaskIMSI: true}
		h := NewVectorHandler(mockUC, cfg)

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
	})

	t.Run("invalid JSON", func(t *testing.T) {
		cfg := &config.Config{LogMaskIMSI: true}
		h := NewVectorHandler(nil, cfg)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		c.Request, _ = http.NewRequest("POST", "/api/v1/vector", bytes.NewBufferString("invalid json"))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(TraceIDKey, "test-trace-id")

		h.HandleVector(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid IMSI", func(t *testing.T) {
		cfg := &config.Config{LogMaskIMSI: true}
		h := NewVectorHandler(nil, cfg)

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
	})

	t.Run("subscriber not found", func(t *testing.T) {
		mockUC := &mockVectorUseCase{
			err: usecase.ErrSubscriberNotFound,
		}
		cfg := &config.Config{LogMaskIMSI: true}
		h := NewVectorHandler(mockUC, cfg)

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
	})
}
