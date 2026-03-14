package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// envelope wraps any value with required metadata for machine consumption.
type envelope struct {
	Meta EnvelopeMeta `json:"meta"`
	Data any          `json:"data"`
}

// EnvelopeMeta carries schema metadata included in every JSON response.
type EnvelopeMeta struct {
	SchemaVersion int `json:"schema_version"`
}

// WriteJSON serialises v to w as an indented JSON envelope with schema_version.
//
// The output always contains:
//
//	{
//	  "meta": { "schema_version": 1 },
//	  "data": <v>
//	}
//
// No ANSI escape codes are ever included. Returns a wrapped error on failure.
func WriteJSON(w io.Writer, v any) error {
	env := envelope{
		Meta: EnvelopeMeta{SchemaVersion: 1},
		Data: v,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(env); err != nil {
		return fmt.Errorf("write json: %w", err)
	}
	return nil
}

// errorJSON is the data payload for JSON error responses.
type errorJSON struct {
	Provider string `json:"provider"`
	Error    string `json:"error"`
}

// WriteErrorJSON writes a structured JSON error to w so programmatic callers
// can parse failure details from stdout even on non-zero exit (issue #4).
func WriteErrorJSON(w io.Writer, provider, errMsg string) error {
	return WriteJSON(w, errorJSON{Provider: provider, Error: errMsg})
}
