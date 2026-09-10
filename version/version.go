// Package version holds build-time metadata injected via -ldflags at compile
// time (see the top-level Makefile). Do not set these defaults expecting
// them to be accurate for a released binary — they exist only as a sane
// fallback for `go run` / `go build` without the Makefile (e.g. local dev
// iteration, `go install`).
package version

var (
	// Version is the git tag/describe output, e.g. "v0.3.0" or
	// "v0.3.0-3-gabc1234-dirty". Set via -X at build time.
	Version = "dev"

	// Commit is the short git commit hash the binary was built from.
	Commit = "none"

	// BuildDate is the UTC build timestamp in RFC3339 format.
	BuildDate = "unknown"
)

// String returns a single-line human-readable version string, suitable for
// `homelabctl version` / `homelabctld --version` output.
func String() string {
	return Version + " (commit " + Commit + ", built " + BuildDate + ")"
}

// Info is a structured form of the same data, for JSON output
// (`homelabctl version --json`) or exposing over the API
// (e.g. included in GET /v1/status or a dedicated /v1/version route).
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// Get returns the current build info as a struct.
func Get() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
	}
}
