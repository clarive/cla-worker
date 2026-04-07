package version

var Version = "dev"
var Commit = "unknown"

// claude: String returns the version string in the format
// "2.0.3 (a3b4c7f)" or "2.0.3 (a3b4c7f+20260327123202)"
// when built from a dirty worktree.
func String() string {
	return Version + " (" + Commit + ")"
}
