package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"conflux/pkg/version"
)

var (
	shortVersion bool
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long: `Display the version information for Conflux including build details.
	
The version command shows the current version of Conflux along with build
information such as Git commit, build date, Go version, and platform.`,
	Example: `  conflux version        # Show full version information
  conflux version --short # Show only version number`,
	RunE: runVersion,
}

func runVersion(cmd *cobra.Command, args []string) error {
	buildInfo := version.Get()

	if shortVersion {
		fmt.Println(buildInfo.Version)
	} else {
		fmt.Println(buildInfo.String())
	}

	return nil
}

func init() {
	rootCmd.AddCommand(versionCmd)

	// Local flags for version command
	versionCmd.Flags().BoolVar(&shortVersion, "short", false, "show only version number")
}
