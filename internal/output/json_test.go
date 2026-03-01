package output_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/indrasvat/dootsabha/internal/output"
)

// TestWriteJSON_ValidJSON verifies that WriteJSON produces valid JSON.
func TestWriteJSON_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := output.WriteJSON(&buf, map[string]string{"key": "value"}); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\nraw: %s", err, buf.String())
	}
}

// TestWriteJSON_SchemaVersion verifies that every response includes meta.schema_version=1.
func TestWriteJSON_SchemaVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := output.WriteJSON(&buf, "ping"); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	meta, ok := result["meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing or wrong type for 'meta' field; got %T", result["meta"])
	}

	sv, ok := meta["schema_version"].(float64)
	if !ok {
		t.Fatalf("missing or wrong type for meta.schema_version; got %T", meta["schema_version"])
	}
	if int(sv) != 1 {
		t.Errorf("expected meta.schema_version=1, got %d", int(sv))
	}
}

// TestWriteJSON_NoANSI verifies that the JSON output contains no ANSI escape codes.
func TestWriteJSON_NoANSI(t *testing.T) {
	var buf bytes.Buffer
	data := map[string]string{"provider": "claude", "status": "ready"}
	if err := output.WriteJSON(&buf, data); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	out := buf.String()
	if strings.Contains(out, "\033[") {
		t.Errorf("JSON output contains ANSI escape codes:\n%s", out)
	}
}

// TestWriteJSON_DataField verifies that the original value is nested under "data".
func TestWriteJSON_DataField(t *testing.T) {
	var buf bytes.Buffer
	if err := output.WriteJSON(&buf, 42); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	data, ok := result["data"]
	if !ok {
		t.Fatal("missing 'data' field in JSON output")
	}
	if v, ok := data.(float64); !ok || int(v) != 42 {
		t.Errorf("expected data=42, got %v", data)
	}
}
