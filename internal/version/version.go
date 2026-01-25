package version

// These variables are set at build time using ldflags.
// Example: go build -ldflags "-X github.com/pablasso/rafa/internal/version.Version=v1.0.0"
var (
	// Version is the semantic version of the application.
	Version = "dev"

	// CommitSHA is the git commit SHA at build time.
	CommitSHA = "unknown"

	// BuildDate is the date when the binary was built.
	BuildDate = "unknown"
)
