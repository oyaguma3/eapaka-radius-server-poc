package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	// テスト時はGinをテストモードに設定
	gin.SetMode(gin.TestMode)
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	problem := BadRequest("invalid parameter")
	WriteError(c, problem)

	// ステータスコード確認
	if w.Code != http.StatusBadRequest {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusBadRequest)
	}

	// Content-Type確認
	contentType := w.Header().Get("Content-Type")
	if contentType != ContentType {
		t.Errorf("Content-Type = %q, want %q", contentType, ContentType)
	}

	// レスポンスボディ確認
	var parsed ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if parsed.Status != http.StatusBadRequest {
		t.Errorf("Response Status = %d, want %d", parsed.Status, http.StatusBadRequest)
	}
	if parsed.Detail != "invalid parameter" {
		t.Errorf("Response Detail = %q, want %q", parsed.Detail, "invalid parameter")
	}
}

func TestAbortWithError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	problem := NotFound("resource not found")
	AbortWithError(c, problem)

	// ステータスコード確認
	if w.Code != http.StatusNotFound {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusNotFound)
	}

	// Abortされたことを確認
	if !c.IsAborted() {
		t.Error("Context should be aborted")
	}

	// Content-Type確認
	contentType := w.Header().Get("Content-Type")
	if contentType != ContentType {
		t.Errorf("Content-Type = %q, want %q", contentType, ContentType)
	}

	// レスポンスボディ確認
	var parsed ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if parsed.Status != http.StatusNotFound {
		t.Errorf("Response Status = %d, want %d", parsed.Status, http.StatusNotFound)
	}
}

func TestWriteErrorInHandler(t *testing.T) {
	router := gin.New()
	router.GET("/test", func(c *gin.Context) {
		problem := InternalServerError("database error")
		WriteError(c, problem)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var parsed ProblemDetail
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if parsed.Title != "Internal Server Error" {
		t.Errorf("Title = %q, want %q", parsed.Title, "Internal Server Error")
	}
}

func TestAbortWithErrorInMiddleware(t *testing.T) {
	router := gin.New()

	// エラーを返すミドルウェア
	router.Use(func(c *gin.Context) {
		if c.Query("error") == "true" {
			problem := BadRequest("validation failed")
			AbortWithError(c, problem)
			return
		}
		c.Next()
	})

	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// エラーケース
	t.Run("with error", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test?error=true", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusBadRequest)
		}

		var parsed ProblemDetail
		if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}

		if parsed.Detail != "validation failed" {
			t.Errorf("Detail = %q, want %q", parsed.Detail, "validation failed")
		}
	})

	// 正常ケース
	t.Run("without error", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
		}
	})
}
