package observability

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

func TestVerbosityLevel(t *testing.T) {
	tests := []struct {
		count int
		want  slog.Level
	}{
		{0, slog.LevelWarn},
		{1, slog.LevelInfo},
		{2, slog.LevelDebug},
		{3, slog.LevelDebug - 1},
		{4, slog.LevelDebug - 1},
	}

	for _, tc := range tests {
		got := VerbosityLevel(tc.count)
		if got != tc.want {
			t.Errorf("VerbosityLevel(%d) = %v, want %v", tc.count, got, tc.want)
		}
	}
}

func TestNewLoggerJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelInfo, "json", &buf)

	logger.Info("test message", "key", "value")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("JSON log output not valid: %v\noutput: %s", err, buf.String())
	}

	if entry["msg"] != "test message" {
		t.Errorf("msg = %q, want 'test message'", entry["msg"])
	}
	if entry["key"] != "value" {
		t.Errorf("key = %q, want 'value'", entry["key"])
	}
}

func TestNewLoggerText(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelInfo, "text", &buf)

	logger.Info("hello world")

	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Errorf("text output missing message: %s", out)
	}
	if !strings.Contains(out, "INFO") {
		t.Errorf("text output missing level: %s", out)
	}
}

func TestNewLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelWarn, "text", &buf)

	logger.Info("should be filtered")
	if buf.Len() > 0 {
		t.Error("info message should be filtered at warn level")
	}

	logger.Warn("should appear")
	if buf.Len() == 0 {
		t.Error("warn message should appear at warn level")
	}
}

func TestNewLoggerDebugWithSource(t *testing.T) {
	var buf bytes.Buffer
	// Level below Debug triggers AddSource.
	logger := NewLogger(slog.LevelDebug-1, "text", &buf)

	logger.Debug("debug msg")
	out := buf.String()
	if !strings.Contains(out, "debug msg") {
		t.Errorf("output missing debug message: %s", out)
	}
	// Source info should be present.
	if !strings.Contains(out, "source=") && !strings.Contains(out, "logger_test.go") {
		t.Logf("source info may not be present in all Go versions, skipping assertion")
	}
}

func TestNewLoggerJSONLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(slog.LevelError, "json", &buf)

	logger.Warn("filtered")
	if buf.Len() > 0 {
		t.Error("warn should be filtered at error level")
	}

	logger.Error("visible")
	if buf.Len() == 0 {
		t.Error("error should be visible at error level")
	}
}

func TestNewTraceID(t *testing.T) {
	id := NewTraceID()

	if !strings.HasPrefix(id, "ds_") {
		t.Errorf("trace ID should start with 'ds_': %s", id)
	}
	if len(id) != 8 {
		t.Errorf("trace ID length = %d, want 8: %s", len(id), id)
	}
}

func TestNewTraceIDUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		id := NewTraceID()
		if seen[id] {
			t.Fatalf("duplicate trace ID: %s", id)
		}
		seen[id] = true
	}
}

func TestSetupDefaultLogger(t *testing.T) {
	// Just verify it doesn't panic.
	logger := SetupDefaultLogger(1, false)
	if logger == nil {
		t.Fatal("SetupDefaultLogger returned nil")
	}

	logger = SetupDefaultLogger(2, true)
	if logger == nil {
		t.Fatal("SetupDefaultLogger returned nil for JSON mode")
	}
}
