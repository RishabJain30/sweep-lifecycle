// Package version holds Sweep's build information. Version, Commit, and
// BuildDate are package-level variables (not constants) because release
// builds overwrite them at compile time via linker flags:
//
//	go build -ldflags "\
//	  -X github.com/RishabJain30/sweep-lifecycle/internal/version.Version=v0.1.0 \
//	  -X github.com/RishabJain30/sweep-lifecycle/internal/version.Commit=<sha> \
//	  -X github.com/RishabJain30/sweep-lifecycle/internal/version.BuildDate=<date>"
//
// A plain `go build` or `go run` never sets them, so they keep these
// development defaults.
package version

import "fmt"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// String renders the current build's version information as a single
// deterministic line. Both `sweep --version` and `sweep version` call this
// same function, so the two entry points can never disagree.
func String() string {
	return fmt.Sprintf("sweep version %s (commit %s, built %s)", Version, Commit, BuildDate)
}
