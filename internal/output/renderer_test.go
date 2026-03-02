package output_test

import (
	"os"
	"testing"

	"github.com/indrasvat/dootsabha/internal/output"
)

// openDevNull returns an *os.File pointing to /dev/null (non-TTY).
func openDevNull(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open /dev/null: %v", err)
	}
	t.Cleanup(func() {
		if err := f.Close(); err != nil {
			t.Errorf("close /dev/null: %v", err)
		}
	})
	return f
}

// TestRenderContext_PipedMode verifies that a /dev/null fd yields IsTTY=false, HasColor=false.
func TestRenderContext_PipedMode(t *testing.T) {
	f := openDevNull(t)
	rc := output.NewRenderContext(f, false)

	if rc.IsTTY {
		t.Error("expected IsTTY=false for /dev/null fd")
	}
	if rc.HasColor {
		t.Error("expected HasColor=false when not a TTY")
	}
	if rc.Format != "text" {
		t.Errorf("expected Format=text, got %q", rc.Format)
	}
}

// TestRenderContext_JSONFlag verifies that jsonFlag=true sets Format to "json".
func TestRenderContext_JSONFlag(t *testing.T) {
	f := openDevNull(t)
	rc := output.NewRenderContext(f, true)

	if rc.Format != "json" {
		t.Errorf("expected Format=json, got %q", rc.Format)
	}
	if !rc.IsJSON() {
		t.Error("expected IsJSON()=true")
	}
}

// TestRenderContext_NOCOLORPresence verifies that the presence of NO_COLOR (even empty) disables color.
func TestRenderContext_NOCOLORPresence(t *testing.T) {
	// Set NO_COLOR to empty string — presence matters, not value.
	t.Setenv("NO_COLOR", "")

	f := openDevNull(t)
	rc := output.NewRenderContext(f, false)

	// Even if somehow IsTTY was true, HasColor must be false.
	if rc.HasColor {
		t.Error("expected HasColor=false when NO_COLOR is set (even to empty string)")
	}
}

// TestRenderContext_COLUMNSOverride verifies that COLUMNS env var overrides width detection.
func TestRenderContext_COLUMNSOverride(t *testing.T) {
	t.Setenv("COLUMNS", "120")

	f := openDevNull(t)
	rc := output.NewRenderContext(f, false)

	if rc.Width != 120 {
		t.Errorf("expected Width=120 from COLUMNS override, got %d", rc.Width)
	}
}

// TestRenderContext_COLUMNSFloor verifies that very narrow COLUMNS values are floored at 40.
func TestRenderContext_COLUMNSFloor(t *testing.T) {
	t.Setenv("COLUMNS", "20")

	f := openDevNull(t)
	rc := output.NewRenderContext(f, false)

	if rc.Width != 40 {
		t.Errorf("expected Width floored at 40 for COLUMNS=20, got %d", rc.Width)
	}
}

// TestRenderContext_DefaultWidth verifies that an unknown terminal width falls back to 80.
func TestRenderContext_DefaultWidth(t *testing.T) {
	// Unset COLUMNS so we fall through to term.GetSize, which will fail on /dev/null.
	if err := os.Unsetenv("COLUMNS"); err != nil {
		t.Fatalf("unsetenv COLUMNS: %v", err)
	}

	f := openDevNull(t)
	rc := output.NewRenderContext(f, false)

	if rc.Width != 80 {
		t.Errorf("expected default Width=80 for non-TTY fd, got %d", rc.Width)
	}
}

// TestProviderDot_PipedMode verifies that ProviderDot returns plain "*" when HasColor=false.
func TestProviderDot_PipedMode(t *testing.T) {
	f := openDevNull(t)
	rc := output.NewRenderContext(f, false)

	dot := output.ProviderDot(rc, output.ClaudeColor)
	if dot != "*" {
		t.Errorf("expected plain * in piped mode, got %q", dot)
	}
}

// TestStatusIndicators_PipedMode verifies OK/FAIL text in piped (non-TTY) mode.
func TestStatusIndicators_PipedMode(t *testing.T) {
	f := openDevNull(t)
	rc := output.NewRenderContext(f, false)

	if got := output.StatusOK(rc); got != "OK" {
		t.Errorf("StatusOK piped: expected %q, got %q", "OK", got)
	}
	if got := output.StatusFail(rc); got != "FAIL" {
		t.Errorf("StatusFail piped: expected %q, got %q", "FAIL", got)
	}
}
