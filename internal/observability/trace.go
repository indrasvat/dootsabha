package observability

import (
	"crypto/rand"
	"fmt"
)

// NewTraceID generates a session trace ID in the format "ds_{random5}".
// Uses crypto/rand for uniqueness.
func NewTraceID() string {
	b := make([]byte, 3) // 3 bytes → 5 hex chars (truncated)
	if _, err := rand.Read(b); err != nil {
		return "ds_00000"
	}
	return fmt.Sprintf("ds_%05x", b)[:8] // "ds_" + 5 hex chars
}
