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
	force    bool
	noCache  bool
	docsDir  string
	spaceKey string
)

// syncCmd represents the sync command
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync local markdown files to Confluence with change detection",
	Long: `Sync local markdown files to Confluence pages with intelligent change detection.

The sync command analyzes your markdown files, detects what has changed since the last sync,
and displays a preview of what will be updated before proceeding. It maintains a local cache
to track file changes and only updates pages that have actually been modified.`,
	Example: `  conflux sync                                # Sync with change detection and confirmation
  conflux sync --dry-run                      # Preview changes without syncing  
  conflux sync --force                        # Skip confirmation prompts
  conflux sync --no-cache                     # Ignore cache, check all files
  conflux sync -docs ./documentation         # Sync specific directory
  conflux sync -space DOCS -v                # Sync to specific space with verbose logging`,
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

	// Handle no-cache flag
	if noCache {
		if err := syncer.ClearCache(); err != nil {
			log.Debug("Could not clear cache: %v", err)
		}
	}

	if err := syncer.Sync(dryRun, force); err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	fmt.Println("Sync completed successfully!")
	return nil
}

func init() {
	rootCmd.AddCommand(syncCmd)

	// Local flags for sync command
	syncCmd.Flags().BoolVar(&dryRun, "dry-run", false, "perform a dry run without making changes")
	syncCmd.Flags().BoolVar(&force, "force", false, "skip confirmation prompts and proceed with sync")
	syncCmd.Flags().BoolVar(&noCache, "no-cache", false, "ignore cached change detection and check all files")
	syncCmd.Flags().StringVarP(&docsDir, "docs", "d", ".", "path to local markdown documents directory")
	syncCmd.Flags().StringVarP(&spaceKey, "space", "s", "", "Confluence space key (overrides config file)")
}
