package format

import (
	"strings"
	"testing"
	"time"
)

func TestRFC3339(t *testing.T) {
	// 2024-01-15 12:00:00 UTC
	unixSec := int64(1705320000)
	result := RFC3339(unixSec)

	if !strings.HasPrefix(result, "2024-01-15T12:00:00") {
		t.Errorf("RFC3339() = %q, expected to start with 2024-01-15T12:00:00", result)
	}
}

func TestDateTime(t *testing.T) {
	// Use a known timestamp and verify format
	unixSec := int64(1705320000)
	result := DateTime(unixSec)

	// Should be in format "2006-01-02 15:04:05"
	_, err := time.Parse("2006-01-02 15:04:05", result)
	if err != nil {
		t.Errorf("DateTime() = %q, not in expected format: %v", result, err)
	}
}

func TestDateTimeShort(t *testing.T) {
	unixSec := int64(1705320000)
	result := DateTimeShort(unixSec)

	// Should be in format "01-02 15:04"
	_, err := time.Parse("01-02 15:04", result)
	if err != nil {
		t.Errorf("DateTimeShort() = %q, not in expected format: %v", result, err)
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		seconds int64
		want    string
	}{
		{0, "0s"},
		{30, "30s"},
		{60, "1m 0s"},
		{90, "1m 30s"},
		{3600, "1h 0m 0s"},
		{3661, "1h 1m 1s"},
		{86400, "24h 0m 0s"},
		{-1, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := Duration(tt.seconds); got != tt.want {
				t.Errorf("Duration(%d) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestDurationShort(t *testing.T) {
	tests := []struct {
		seconds int64
		want    string
	}{
		{0, "0:00"},
		{30, "0:30"},
		{60, "1:00"},
		{90, "1:30"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
		{-1, "-"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := DurationShort(tt.seconds); got != tt.want {
				t.Errorf("DurationShort(%d) = %q, want %q", tt.seconds, got, tt.want)
			}
		})
	}
}

func TestElapsed(t *testing.T) {
	// Start time 1 hour ago
	startTime := time.Now().Unix() - 3600
	result := Elapsed(startTime)

	// Should be approximately "1h 0m 0s" (allow some tolerance)
	if !strings.HasPrefix(result, "1h") {
		t.Errorf("Elapsed() = %q, expected to start with '1h'", result)
	}
}
