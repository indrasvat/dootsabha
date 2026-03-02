package core

import (
	"testing"
)

func TestExitCodeConstants(t *testing.T) {
	if ExitSuccess != 0 {
		t.Errorf("ExitSuccess = %d, want 0", ExitSuccess)
	}
	if ExitError != 1 {
		t.Errorf("ExitError = %d, want 1", ExitError)
	}
	if ExitUsage != 2 {
		t.Errorf("ExitUsage = %d, want 2", ExitUsage)
	}
	if ExitProvider != 3 {
		t.Errorf("ExitProvider = %d, want 3", ExitProvider)
	}
	if ExitTimeout != 4 {
		t.Errorf("ExitTimeout = %d, want 4", ExitTimeout)
	}
	if ExitPartial != 5 {
		t.Errorf("ExitPartial = %d, want 5", ExitPartial)
	}
}

func TestHighestExitCodePrecedence(t *testing.T) {
	tests := []struct {
		name  string
		codes []int
		want  int
	}{
		{"empty", nil, 0},
		{"single success", []int{0}, 0},
		{"single error", []int{1}, 1},
		{"single usage", []int{2}, 2},
		{"single provider", []int{3}, 3},
		{"single timeout", []int{4}, 4},
		{"single partial", []int{5}, 5},

		// Pairwise precedence: 2 > 4 > 3 > 5 > 1 > 0
		{"usage beats timeout", []int{2, 4}, 2},
		{"usage beats provider", []int{2, 3}, 2},
		{"usage beats partial", []int{2, 5}, 2},
		{"usage beats error", []int{2, 1}, 2},
		{"usage beats success", []int{2, 0}, 2},

		{"timeout beats provider", []int{4, 3}, 4},
		{"timeout beats partial", []int{4, 5}, 4},
		{"timeout beats error", []int{4, 1}, 4},
		{"timeout beats success", []int{4, 0}, 4},

		{"provider beats partial", []int{3, 5}, 3},
		{"provider beats error", []int{3, 1}, 3},
		{"provider beats success", []int{3, 0}, 3},

		{"partial beats error", []int{5, 1}, 5},
		{"partial beats success", []int{5, 0}, 5},

		{"error beats success", []int{1, 0}, 1},

		// Multi-code scenarios
		{"timeout + partial = timeout", []int{4, 5}, 4},
		{"provider + partial + error = provider", []int{3, 5, 1}, 3},
		{"all codes = usage", []int{0, 1, 2, 3, 4, 5}, 2},
		{"all except usage = timeout", []int{0, 1, 3, 4, 5}, 4},

		// Order independence
		{"reverse order", []int{0, 1, 5, 3, 4, 2}, 2},
		{"sorted order", []int{0, 1, 2, 3, 4, 5}, 2},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := HighestExitCode(tc.codes...)
			if got != tc.want {
				t.Errorf("HighestExitCode(%v) = %d, want %d", tc.codes, got, tc.want)
			}
		})
	}
}

func TestHighestExitCodeDuplicates(t *testing.T) {
	got := HighestExitCode(3, 3, 3)
	if got != 3 {
		t.Errorf("HighestExitCode(3,3,3) = %d, want 3", got)
	}
}
