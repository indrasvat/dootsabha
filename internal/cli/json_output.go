package cli

import (
	"os"

	"github.com/indrasvat/dootsabha/internal/output"
)

// jsonDocWritten records that a command has already emitted a JSON document on
// stdout.
//
// Commands render their own envelope (result, or a detailed provider error), and
// Execute() separately emits an error envelope for any ExitError in JSON mode.
// Without coordination both fire on a failure path and stdout carries TWO JSON
// documents, which `jq`/`json.load` reject with "Extra data" — breaking scripted
// consumers exactly when they most need to read the failure. The command's own
// document wins because it is the more specific one (it names the provider, and
// for council it carries the partial results).
//
// Safe as package state: one dootsabha command runs per process.
var jsonDocWritten bool

// emitJSON writes v to stdout as the command's single JSON document.
func emitJSON(v any) error {
	jsonDocWritten = true
	return output.WriteJSON(os.Stdout, v)
}

// emitErrorJSON writes a structured error as the command's single JSON document.
func emitErrorJSON(provider, errMsg string) {
	jsonDocWritten = true
	_ = output.WriteErrorJSON(os.Stdout, provider, errMsg)
}

// markJSONWritten records a JSON document written directly (e.g. via
// json.NewEncoder) rather than through the helpers above.
func markJSONWritten() { jsonDocWritten = true }
