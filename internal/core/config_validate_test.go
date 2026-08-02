package core_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/indrasvat/dootsabha/internal/core"
)

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

// Viper coerces unparseable values to zero values, so a plausible typo silently
// changed behaviour instead of being reported. The contract says exit 6 covers a
// config that is "missing, unreadable, or INVALID" — invalid values are invalid.
func TestLoadConfigRejectsUnparseableDuration(t *testing.T) {
	for _, key := range []string{"timeout", "session_timeout"} {
		path := writeCfg(t, key+`: "not a duration"`+"\n")
		_, err := core.LoadConfig(path)
		if err == nil {
			t.Errorf("%s with a non-duration value must be rejected, got nil", key)
			continue
		}
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error %q should name the offending key %q", err, key)
		}
	}
}

// `timeout: 5 minutes` is the typo this protects against — YAML reads it as a
// string, viper coerces to 0, and the run silently used the built-in default.
func TestLoadConfigRejectsHumanDuration(t *testing.T) {
	if _, err := core.LoadConfig(writeCfg(t, "timeout: 5 minutes\n")); err == nil {
		t.Error("`timeout: 5 minutes` must be rejected rather than silently defaulted")
	}
}

func TestLoadConfigAcceptsValidDurations(t *testing.T) {
	cfg, err := core.LoadConfig(writeCfg(t, "timeout: 90s\nsession_timeout: 30m\n"))
	if err != nil {
		t.Fatalf("valid durations must load: %v", err)
	}
	if cfg.Timeout.Seconds() != 90 {
		t.Errorf("timeout = %v, want 90s", cfg.Timeout)
	}
}

// A provider entry must be a map; a scalar there is a malformed config, not a
// broken provider to be reported as degraded.
func TestLoadConfigRejectsNonMapProvider(t *testing.T) {
	path := writeCfg(t, "providers:\n  claude: \"not a map\"\n")
	_, err := core.LoadConfig(path)
	if err == nil {
		t.Fatal("a scalar provider entry must be rejected")
	}
	if !strings.Contains(err.Error(), "claude") {
		t.Errorf("error %q should name the offending provider", err)
	}
}

func TestLoadConfigRejectsNonIntRounds(t *testing.T) {
	if _, err := core.LoadConfig(writeCfg(t, "council:\n  rounds: \"three\"\n")); err == nil {
		t.Error("a non-numeric rounds value must be rejected")
	}
}

// Reading an unbounded stream (character device, FIFO) never returns. A config
// must be a regular file; `--config /dev/zero` used to hang forever.
func TestLoadConfigRejectsNonRegularFile(t *testing.T) {
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skip("/dev/zero unavailable")
	}
	done := make(chan error, 1)
	go func() { _, err := core.LoadConfig("/dev/zero"); done <- err }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("a character device must be rejected, not read")
		}
		if !strings.Contains(err.Error(), "regular file") {
			t.Errorf("error %q should explain that a config must be a regular file", err)
		}
	case <-timeoutAfter():
		t.Fatal("LoadConfig hung reading a character device")
	}
}

func TestLoadConfigRejectsDirectory(t *testing.T) {
	if _, err := core.LoadConfig(t.TempDir()); err == nil {
		t.Error("a directory must be rejected as a config file")
	}
}

func timeoutAfter() <-chan time.Time { return time.After(5 * time.Second) }
