package version

// Version, Commit, and Date are set via -ldflags at build time.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// ShortCommit returns the first 7 characters of the commit hash.
func ShortCommit() string {
	if len(Commit) > 7 {
		return Commit[:7]
	}
	return Commit
}

// ShortDate returns the date portion (before 'T') of an ISO timestamp.
func ShortDate() string {
	for i := range Date {
		if Date[i] == 'T' {
			return Date[:i]
		}
	}
	return Date
}

// String returns the full version string.
func String() string {
	return Version + " (" + ShortCommit() + ") built " + ShortDate()
}
