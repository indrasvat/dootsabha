package version

// Version, Commit, and Date are set via -ldflags at build time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns the full version string.
func String() string {
	return Version + " (" + Commit + ") built " + Date
}
