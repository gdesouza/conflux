package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configFile string
	verbose    bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "conflux",
	Short: "Sync local markdown files to Confluence",
	Long: `Conflux is a tool for synchronizing local markdown files to Confluence pages.
It provides commands to sync documentation and list page hierarchies in Confluence spaces.`,
	Example: `  conflux sync                                    # Sync current directory
  conflux sync -docs ./docs -dry-run             # Sync with options
  conflux list-pages -space DOCS                 # List all pages
  conflux list-pages -space DOCS -parent "API"   # List under parent`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global persistent flags available to all subcommands
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "config.yaml", "path to configuration file")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose logging")
}
