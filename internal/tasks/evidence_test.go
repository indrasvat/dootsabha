// Package tasks holds repo-state guards over docs/tasks/. It has no non-test
// source: these are assertions about the repository, not code that ships.
//
// This is a TEST, not a hook, and that is the point. Its predecessor —
// scripts/verify-visual-tests.sh, wired to a Claude Code hook — never ran: it was
// registered nowhere, read positional args where the hook contract passes JSON on
// stdin, and its checks used `grep -oP`, which BSD grep does not support, behind
// a `|| true`. Three independent failures, none visible, because hooks do not run
// in CI. "Does this file contain evidence?" is a question about repo STATE, so it
// belongs where state questions belong: a test that fails the build.
package tasks

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const tasksDir = "../../docs/tasks"

// grandfathered lists tasks that were DONE before this check existed. It is an
// honest exemption, not an oversight: most predate shux entirely, and their L4
// evidence was iTerm2 screenshots in a gitignored directory that no longer
// exists. Writing evidence sections for them now would mean inventing evidence
// for work whose proof is gone — the precise dishonesty this check exists to
// prevent.
//
// The list may only SHRINK. Removing an entry is how a task gets held to the
// current standard; adding one is a deliberate act a reviewer sees. Stale
// entries fail TestGrandfatherListIsCurrent.
var grandfathered = map[string]struct{}{
	"000-codex-jsonl-spike.md":           {},
	"001-claude-json-spike.md":           {},
	"002-gemini-json-spike.md":           {},
	"003-go-plugin-grpc-spike.md":        {},
	"004-subprocess-management-spike.md": {},
	"005-cobra-alias-spike.md":           {},
	"006-terminal-ux-spike.md":           {},
	"007-pty-pipe-spike.md":              {},
	"100-project-scaffold.md":            {},
	"101-render-context.md":              {},
	"102-config-manager.md":              {},
	"103-subprocess-runner.md":           {},
	"104-claude-provider.md":             {},
	"105-codex-gemini-providers.md":      {},
	"106-cli-wiring.md":                  {},
	"107-status-bugfix.md":               {},
	"108-align-arch-doc.md":              {},
	"200-parallel-dispatch.md":           {},
	"201-peer-review.md":                 {},
	"202-synthesis.md":                   {},
	"203-review-command.md":              {},
	"207-output-polish.md":               {},
	"301-plugin-manager.md":              {},
	"302-extract-providers.md":           {},
	"303-council-strategy-plugin.md":     {},
	"304-extension-discovery.md":         {},
	"305-plugin-command.md":              {},
	"400-structured-logging.md":          {},
	"401-metrics-collection.md":          {},
	"402-edge-cases.md":                  {},
	"403-context-file.md":                {},
	"404-l5-acceptance.md":               {},
	"500-readme.md":                      {},
	"501-default-config.md":              {},
	"502-skill.md":                       {},
	"503-build-release.md":               {},
	"504-final-acceptance.md":            {},
	"600-recap-extension.md":             {},
	"700-council-json-robustness.md":     {},
	"701-install-script.md":              {},
	"702-codex-default-model.md":         {},
	"704-grok-provider.md":               {},
	"705-timeout-scoping.md":             {},
}

var (
	doneRe = regexp.MustCompile(`(?mi)^\W*\**Status:\**\s*\**\s*DONE\b`)
	// `\b` will not do here: markdown emphasis wraps it as `_N/A_`, and `_` is a
	// word character, so there is no boundary after the A.
	naRe      = regexp.MustCompile(`(?mi)^\s*[_*]*N/A(?:[^A-Za-z0-9]|$)|not applicable|no provider CLIs|nothing output-visible`)
	scriptRe  = regexp.MustCompile(`\.shux/scripts/[A-Za-z0-9._-]+\.sh`)
	headingRe = regexp.MustCompile(`(?m)^## `)
)

// evidenceSection returns the body of "## Visual Test Results", bounded by the
// NEXT level-2 heading. Reading to end-of-file let a later section stand in as
// evidence: an empty visual section passed when unrelated prose further down
// happened to mention "no provider CLIs".
func evidenceSection(doc string) (string, bool) {
	const heading = "## Visual Test Results"
	_, rest, ok := strings.Cut(doc, heading)
	if !ok {
		return "", false
	}
	if loc := headingRe.FindStringIndex(rest); loc != nil {
		rest = rest[:loc[0]]
	}
	return rest, true
}

// checkEvidence reports why a task file's evidence is inadequate, or nil.
//
// It deliberately does NOT judge quality. A check can confirm a capture script
// exists; it cannot confirm anyone looked at the frames. Claiming otherwise is
// what let the old gate be called "the single most important mechanism for
// preventing agent hallucinations" while doing nothing at all.
func checkEvidence(doc string, scriptExists func(string) bool) []string {
	if !doneRe.MatchString(doc) {
		return nil // Only a claim of completion is held to this.
	}
	section, ok := evidenceSection(doc)
	if !ok {
		return []string{"marked DONE with no '## Visual Test Results' section"}
	}

	// L4 cannot be captured where no provider CLI exists — a cloud session — and
	// some tasks render nothing at all. Saying so is allowed; inventing evidence
	// is not. Such a section is judged on its REASON, not its length: one line can
	// be a complete answer, while five lines of padding is not.
	if naRe.MatchString(section) {
		if len(strings.Fields(section)) < 12 {
			return []string{"L4 marked N/A without a reason — say why it could not be captured"}
		}
		return nil
	}

	var problems []string
	if len(strings.Fields(section)) < 20 {
		problems = append(problems, "Visual Test Results section is too thin to be real evidence")
	}
	scripts := scriptRe.FindAllString(section, -1)
	if len(scripts) == 0 {
		problems = append(problems, "no .shux/scripts/*.sh capture script referenced — L4 is shux (CLAUDE.md)")
	}
	seen := map[string]bool{}
	for _, s := range scripts {
		if seen[s] {
			continue
		}
		seen[s] = true
		if !scriptExists(s) {
			problems = append(problems, "capture script referenced but missing: "+s)
		}
	}
	return problems
}

// Every task claiming DONE must carry evidence, or a reasoned exemption.
func TestDoneTasksCarryEvidence(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(tasksDir, "*.md"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("no task files found — is tasksDir wrong?")
	}

	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join("..", "..", rel))
		return err == nil
	}

	for _, file := range files {
		doc, err := os.ReadFile(file) //nolint:gosec // repo-local test fixture path
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		base := filepath.Base(file)
		if _, old := grandfathered[base]; old {
			continue
		}
		for _, problem := range checkEvidence(string(doc), exists) {
			t.Errorf("%s: %s", base, problem)
		}
	}
}

// The old gate rotted because nothing ever exercised its FAILURE path — it
// reported success for years while checking nothing. These fixtures are the
// difference between a gate and the appearance of one.
func TestEvidenceCheckRejectsWhatItShould(t *testing.T) {
	const good = `## Status: DONE
## Visual Test Results
Captured with ` + "`.shux/scripts/real.sh`" + ` — real CLI, no mocks in any frame.

| Frame | Shows |
|---|---|
| 01 | the premise, straight from the CLI |
| 02 | status reporting the bumped model |
`
	exists := func(s string) bool { return s == ".shux/scripts/real.sh" }

	for _, tc := range []struct {
		name, doc string
		wantFail  string // substring of the expected complaint; "" = must pass
	}{
		{"complete evidence", good, ""},
		{"not done yet is exempt", "## Status: IN PROGRESS\nno evidence at all", ""},
		{"reasoned cloud N/A", `## Status: DONE
## Visual Test Results
_N/A — no provider CLIs are installed in this cloud session, so no live frame
could be captured. L1/L2/L3/L5 ran green against the mock providers._`, ""},
		{"nothing to render is a reason", `## Status: DONE
## Visual Test Results
_N/A — nothing output-visible._ This task generates protobuf code and renders
no terminal output, so there is no frame to capture.`, ""},

		{"done with no section", "## Status: DONE\nall finished!", "no '## Visual Test Results' section"},
		{"bare N/A", "## Status: DONE\n## Visual Test Results\n_N/A_", "without a reason"},
		{"padded but scriptless", `## Status: DONE
## Visual Test Results
We ran it and looked at it and it all seemed completely fine to us honestly,
there is really nothing more to say about the matter at this point in time.`, "no .shux/scripts"},
		{"script referenced but absent", `## Status: DONE
## Visual Test Results
Captured with ` + "`.shux/scripts/ghost.sh`" + ` on the real CLI.

| Frame | Shows |
|---|---|
| 01 | something that was never actually captured here |`, "missing: .shux/scripts/ghost.sh"},
		{"too thin to be evidence", "## Status: DONE\n## Visual Test Results\n`.shux/scripts/real.sh`", "too thin"},

		// The bug the previous implementation shipped: the section ran to EOF, so a
		// LATER section supplied both the exemption wording and the word count.
		{"later section cannot stand in as evidence", `## Status: DONE
## Visual Test Results

## Notes
This work had no provider CLIs available and there is a great deal of further
prose here which would previously have satisfied the word count on its own.`, "no .shux/scripts"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			problems := checkEvidence(tc.doc, exists)
			if tc.wantFail == "" {
				if len(problems) > 0 {
					t.Errorf("expected to pass, got %v", problems)
				}
				return
			}
			if len(problems) == 0 {
				t.Fatalf("expected a complaint containing %q, got none", tc.wantFail)
			}
			if !strings.Contains(strings.Join(problems, "; "), tc.wantFail) {
				t.Errorf("complaints %v do not mention %q", problems, tc.wantFail)
			}
		})
	}
}

// An exemption that outlives its file is dead weight, and a list nobody prunes is
// how a rule quietly becomes optional. Every entry must still name a real task.
func TestGrandfatherListIsCurrent(t *testing.T) {
	for name := range grandfathered {
		if _, err := os.Stat(filepath.Join(tasksDir, name)); err != nil {
			t.Errorf("grandfathered %q no longer exists — drop it from the list", name)
		}
	}
}
