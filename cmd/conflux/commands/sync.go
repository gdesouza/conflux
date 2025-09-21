package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/sync"
	"conflux/pkg/logger"
)

var (
	dryRun   bool
	docsDir  string
	spaceKey string
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local markdown files to Confluence",
	Long: `Sync local markdown files to Confluence pages.

This command reads markdown files from the specified directory and synchronizes
them with Confluence pages based on the configuration file.`,
	Example: `  conflux sync                                # Sync current directory
  conflux sync -docs ./documentation         # Sync specific directory
  conflux sync -docs ./docs -dry-run         # Dry run sync
  conflux sync -space DOCS -v                # Sync to specific space with verbose logging
  conflux sync -config prod-config.yaml -v   # Sync with custom config file`,
	RunE: runSync,
}

func runSync(cmd *cobra.Command, args []string) error {
	log := logger.New(verbose)

	cfg, err := config.LoadForSync(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config markdown directory with CLI flag
	cfg.Local.MarkdownDir = docsDir

	// Override space key if provided via CLI flag
	if spaceKey != "" {
		cfg.Confluence.SpaceKey = spaceKey
	}

	// Validate that space key is available either from config or CLI
	if cfg.Confluence.SpaceKey == "" {
		return fmt.Errorf("space key is required: provide via config file or use --space flag")
	}

	syncer := sync.New(cfg, log)

	if err := syncer.Sync(dryRun); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Println("Sync completed successfully!")
	return nil
}

func init() {
	rootCmd.AddCommand(syncCmd)

	// Local flags for sync command
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "perform a dry run without making changes")
	syncCmd.Flags().StringVarP(&docsDir, "docs", "d", ".", "path to local markdown documents directory")
	syncCmd.Flags().StringVarP(&spaceKey, "space", "s", "", "Confluence space key (overrides config file)")
}
