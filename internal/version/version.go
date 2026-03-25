package version

var Version = "dev"
var Commit = "unknown"

func String() string {
	return Version + " (" + Commit + ")"
}
