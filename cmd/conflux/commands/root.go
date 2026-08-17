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
	Short: "Edit Confluence pages through local Markdown",
	Long: `Conflux provides a safe pull, edit, and push workflow for Confluence pages,
plus commands for configuration and page inspection.`,
	Example: `  conflux pull -s DOCS -p 123 --output ./page.md # Pull editable artifact
  conflux push -f ./page.md                       # Push guarded edits
  conflux push -f ./doc.md -s DOCS               # Create standalone page
  conflux pull -s DOCS -p "My Page" -f markdown  # Download page as markdown
  conflux pages -s DOCS                          # List all pages
  conflux pages show -s DOCS -p "API"            # Show page details`,
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
