package version

import "testing"

func TestShortCommitTruncates(t *testing.T) {
	old := Commit
	t.Cleanup(func() { Commit = old })

	Commit = "abc1234def5678"
	if got := ShortCommit(); got != "abc1234" {
		t.Errorf("ShortCommit() = %q, want %q", got, "abc1234")
	}
}

func TestShortCommitShortInput(t *testing.T) {
	old := Commit
	t.Cleanup(func() { Commit = old })

	Commit = "abc"
	if got := ShortCommit(); got != "abc" {
		t.Errorf("ShortCommit() = %q, want %q", got, "abc")
	}
}

func TestShortCommitExactly7(t *testing.T) {
	old := Commit
	t.Cleanup(func() { Commit = old })

	Commit = "abc1234"
	if got := ShortCommit(); got != "abc1234" {
		t.Errorf("ShortCommit() = %q, want %q", got, "abc1234")
	}
}

func TestShortDateISO(t *testing.T) {
	old := Date
	t.Cleanup(func() { Date = old })

	Date = "2026-03-02T12:00:00Z"
	if got := ShortDate(); got != "2026-03-02" {
		t.Errorf("ShortDate() = %q, want %q", got, "2026-03-02")
	}
}

func TestShortDateNoT(t *testing.T) {
	old := Date
	t.Cleanup(func() { Date = old })

	Date = "unknown"
	if got := ShortDate(); got != "unknown" {
		t.Errorf("ShortDate() = %q, want %q", got, "unknown")
	}
}

func TestString(t *testing.T) {
	oldV, oldC, oldD := Version, Commit, Date
	t.Cleanup(func() { Version, Commit, Date = oldV, oldC, oldD })

	Version = "v1.2.3"
	Commit = "abc1234def"
	Date = "2026-03-02T12:00:00Z"

	want := "v1.2.3 (abc1234) built 2026-03-02"
	if got := String(); got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}
