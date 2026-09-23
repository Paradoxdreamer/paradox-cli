package version

// Version is the current version of the Paradox CLI.
// It is set at build time via ldflags.
var Version = "0.1.0-dev"

// Commit is the git commit hash, set at build time.
var Commit = "none"

// BuildDate is the build date, set at build time.
var BuildDate = "unknown"
