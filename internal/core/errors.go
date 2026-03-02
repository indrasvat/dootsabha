package core

// Exit codes per PRD §6.1.
const (
	ExitSuccess  = 0 // Everything OK
	ExitError    = 1 // General error
	ExitUsage    = 2 // Bad flags, missing args
	ExitProvider = 3 // Provider error (CLI failed, auth invalid)
	ExitTimeout  = 4 // At least one agent timed out
	ExitPartial  = 5 // Partial result (some agents failed)
)

// exitCodePrecedence defines the precedence order for exit codes.
// Higher precedence codes override lower ones.
// Precedence: 2 > 4 > 3 > 5 > 1 > 0
var exitCodePrecedence = map[int]int{
	ExitUsage:    6, // highest
	ExitTimeout:  5,
	ExitProvider: 4,
	ExitPartial:  3,
	ExitError:    2,
	ExitSuccess:  1, // lowest
}

// HighestExitCode returns the exit code with highest precedence from a set.
// Precedence: 2 > 4 > 3 > 5 > 1 > 0.
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
