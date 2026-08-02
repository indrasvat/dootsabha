package core

// Exit codes. Each code maps to exactly one caller action — that is the rule
// that keeps them useful for scripting.
//
//	0  proceed: output is complete and usable
//	1  report a bug: unexpected internal failure
//	2  fix the command: bad flag, missing arg, unknown agent/chair
//	3  retry or pick another agent: every requested agent failed
//	4  raise --timeout or shrink the prompt
//	5  use the output, note the gaps: some agents failed
//	6  fix the config: missing, unreadable, or invalid
const (
	ExitSuccess  = 0 // Everything OK
	ExitError    = 1 // Unexpected internal error
	ExitUsage    = 2 // Bad flags, missing args, unknown agent/chair
	ExitProvider = 3 // Every requested agent failed — nothing usable
	ExitTimeout  = 4 // At least one agent timed out
	ExitPartial  = 5 // Partial result — some agents failed, output still usable
	ExitConfig   = 6 // Config missing, unreadable, or invalid
)

// exitCodePrecedence orders codes when several apply at once.
// Higher wins. Precedence: 2 > 6 > 4 > 3 > 5 > 1 > 0.
//
// Read it most-blocking first: the command was never valid (2), then the config
// could not be loaded (6), then a deadline was hit (4), then every agent failed
// so nothing is usable (3), then some agents failed but output survives (5),
// then an unexpected internal error (1).
var exitCodePrecedence = map[int]int{
	ExitUsage:    7, // highest — the invocation itself was wrong
	ExitConfig:   6,
	ExitTimeout:  5,
	ExitProvider: 4,
	ExitPartial:  3,
	ExitError:    2,
	ExitSuccess:  1, // lowest
}

// HighestExitCode returns the exit code with highest precedence from a set.
// Precedence: 2 > 6 > 4 > 3 > 5 > 1 > 0.
func HighestExitCode(codes ...int) int {
	if len(codes) == 0 {
		return ExitSuccess
	}

	best := codes[0]
	bestPrecedence := exitCodePrecedence[best]

	for _, code := range codes[1:] {
		p := exitCodePrecedence[code]
		if p > bestPrecedence {
			best = code
			bestPrecedence = p
		}
	}

	return best
}
