package buildinfo

import "testing"

func TestStringIncludesBuildMetadata(t *testing.T) {
	originalVersion, originalCommit, originalDate := Version, Commit, Date
	t.Cleanup(func() {
		Version, Commit, Date = originalVersion, originalCommit, originalDate
	})
	Version, Commit, Date = "v1.2.3", "abc1234", "2026-08-28T00:00:00Z"

	got := String()
	want := "oms-platform v1.2.3 (commit abc1234, built 2026-08-28T00:00:00Z)"
	if got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}
