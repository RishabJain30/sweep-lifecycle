package version

import "testing"

func TestStringUsesDevelopmentDefaultsByDefault(t *testing.T) {
	if Version != "dev" {
		t.Fatalf("Version = %q, want %q (test must run before any -ldflags override)", Version, "dev")
	}

	if Commit != "unknown" {
		t.Fatalf("Commit = %q, want %q", Commit, "unknown")
	}

	if BuildDate != "unknown" {
		t.Fatalf("BuildDate = %q, want %q", BuildDate, "unknown")
	}

	want := "sweep version dev (commit unknown, built unknown)"
	if got := String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestStringFormatsInjectedBuildInformation(t *testing.T) {
	originalVersion, originalCommit, originalBuildDate := Version, Commit, BuildDate
	t.Cleanup(func() {
		Version, Commit, BuildDate = originalVersion, originalCommit, originalBuildDate
	})

	Version = "v0.1.0"
	Commit = "abc1234"
	BuildDate = "2026-09-05T00:00:00Z"

	want := "sweep version v0.1.0 (commit abc1234, built 2026-09-05T00:00:00Z)"
	if got := String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
