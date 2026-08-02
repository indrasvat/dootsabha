package core_test

import (
	"testing"

	"github.com/indrasvat/dootsabha/internal/core"
)

// The exit-code scheme is a contract: one code, one caller action.
//
//	0 complete · 1 internal bug · 2 fix the command · 3 retry/other agent
//	4 raise timeout · 5 usable but incomplete · 6 fix the config
func TestExitCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"ExitSuccess", core.ExitSuccess, 0},
		{"ExitError", core.ExitError, 1},
		{"ExitUsage", core.ExitUsage, 2},
		{"ExitProvider", core.ExitProvider, 3},
		{"ExitTimeout", core.ExitTimeout, 4},
		{"ExitPartial", core.ExitPartial, 5},
		{"ExitConfig", core.ExitConfig, 6},
	}
	for _, tc := range tests {
		if tc.got != tc.want {
			t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
		}
	}
}

// Precedence: 2 > 6 > 4 > 3 > 5 > 1 > 0.
//
// Rationale, most-blocking first: the command was never valid (2), then the
// config could not be loaded (6), then a deadline was hit (4), then every agent
// failed so nothing is usable (3), then some agents failed but output survives
// (5), then an unexpected internal error (1).
func TestExitCodePrecedenceOrder(t *testing.T) {
	// Each entry must beat every entry after it.
	order := []int{
		core.ExitUsage,    // 2
		core.ExitConfig,   // 6
		core.ExitTimeout,  // 4
		core.ExitProvider, // 3
		core.ExitPartial,  // 5
		core.ExitError,    // 1
		core.ExitSuccess,  // 0
	}
	for i, hi := range order {
		for _, lo := range order[i+1:] {
			if got := core.HighestExitCode(hi, lo); got != hi {
				t.Errorf("HighestExitCode(%d, %d) = %d, want %d", hi, lo, got, hi)
			}
			if got := core.HighestExitCode(lo, hi); got != hi {
				t.Errorf("HighestExitCode(%d, %d) = %d, want %d (order must not matter)", lo, hi, got, hi)
			}
		}
	}
}

// A config error outranks a partial result: if the config never loaded, any
// "partial" signal is meaningless.
func TestExitConfigBeatsPartial(t *testing.T) {
	if got := core.HighestExitCode(core.ExitPartial, core.ExitConfig); got != core.ExitConfig {
		t.Errorf("HighestExitCode(partial, config) = %d, want %d", got, core.ExitConfig)
	}
}

// Usage beats everything — the command was never valid to begin with.
func TestExitUsageBeatsAll(t *testing.T) {
	for _, other := range []int{core.ExitConfig, core.ExitTimeout, core.ExitProvider, core.ExitPartial, core.ExitError, core.ExitSuccess} {
		if got := core.HighestExitCode(other, core.ExitUsage); got != core.ExitUsage {
			t.Errorf("HighestExitCode(%d, usage) = %d, want %d", other, got, core.ExitUsage)
		}
	}
}

func TestHighestExitCodeEmpty(t *testing.T) {
	if got := core.HighestExitCode(); got != core.ExitSuccess {
		t.Errorf("HighestExitCode() = %d, want %d", got, core.ExitSuccess)
	}
}
