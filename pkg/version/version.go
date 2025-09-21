package version

import (
	"fmt"
	"runtime"
)

// Version information. These will be set by build flags.
var (
	Version   = "dev"
	GitCommit = ""
	BuildDate = ""
)

// BuildInfo contains detailed version information
type BuildInfo struct {
	Version   string
	GitCommit string
	BuildDate string
	GoVersion string
	Platform  string
}

// Get returns the current build information
func Get() BuildInfo {
	return BuildInfo{
		Version:   Version,
		GitCommit: GitCommit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns a formatted version string
func (b BuildInfo) String() string {
	result := fmt.Sprintf("conflux version %s", b.Version)

	if b.GitCommit != "" {
		result += fmt.Sprintf(" (%s)", b.GitCommit)
	}

	if b.BuildDate != "" {
		result += fmt.Sprintf(" built on %s", b.BuildDate)
	}

	result += fmt.Sprintf(" %s %s", b.GoVersion, b.Platform)

	return result
}
