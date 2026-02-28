package backend

import (
	"errors"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/httputil"
)

func TestNewProblemDetail(t *testing.T) {
	pd := httputil.NewProblemDetail(400, "Bad Request", "invalid input")

	if pd.Type != "about:blank" {
		t.Errorf("Type = %q, want %q", pd.Type, "about:blank")
	}
	if pd.Title != "Bad Request" {
		t.Errorf("Title = %q, want %q", pd.Title, "Bad Request")
	}
	if pd.Detail != "invalid input" {
		t.Errorf("Detail = %q, want %q", pd.Detail, "invalid input")
	}
	if pd.Status != 400 {
		t.Errorf("Status = %d, want %d", pd.Status, 400)
	}
}

func TestBackendNotImplementedError(t *testing.T) {
	err := &BackendNotImplementedError{ID: "99"}

	if err.Error() != `backend "99" is not implemented` {
		t.Errorf("Error() = %q, want %q", err.Error(), `backend "99" is not implemented`)
	}

	// errors.As で型判定できることを確認
	var target *BackendNotImplementedError
	if !errors.As(err, &target) {
		t.Error("errors.As failed for BackendNotImplementedError")
	}
	if target.ID != "99" {
		t.Errorf("ID = %q, want %q", target.ID, "99")
	}
}

func TestBackendCommunicationError(t *testing.T) {
	cause := errors.New("connection refused")
	err := &BackendCommunicationError{Err: cause}

	if err.Error() != "backend communication error: connection refused" {
		t.Errorf("Error() = %q", err.Error())
	}

	// Unwrap で元のエラーを取得できることを確認
	if !errors.Is(err, cause) {
		t.Error("errors.Is failed: should unwrap to cause")
	}

	// errors.As で型判定できることを確認
	var target *BackendCommunicationError
	if !errors.As(err, &target) {
		t.Error("errors.As failed for BackendCommunicationError")
	}
}

func TestBackendResponseError(t *testing.T) {
	pd := &httputil.ProblemDetail{
		Type:   "about:blank",
		Title:  "Not Found",
		Detail: "subscriber not found",
		Status: 404,
	}
	err := &BackendResponseError{StatusCode: 404, Problem: pd}

	if err.Error() != "backend returned status 404: subscriber not found" {
		t.Errorf("Error() = %q", err.Error())
	}

	// errors.As で型判定できることを確認
	var target *BackendResponseError
	if !errors.As(err, &target) {
		t.Error("errors.As failed for BackendResponseError")
	}
	if target.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want %d", target.StatusCode, 404)
	}
}
