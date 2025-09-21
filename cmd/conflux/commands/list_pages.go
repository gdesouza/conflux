package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

var (
	space      string
	parentPage string
)

// listPagesCmd represents the list-pages command
var listPagesCmd = &cobra.Command{
	Use:   "list-pages",
	Short: "List page hierarchy from a Confluence space",
	Long: `List page hierarchy from a Confluence space.

This command connects to Confluence and retrieves the page hierarchy for a specified
space. You can optionally specify a parent page to start the hierarchy from.`,
	Example: `  conflux list-pages -space DOCS                     # List all pages in space
  conflux list-pages -space DOCS -parent "API"      # List pages under parent
  conflux list-pages -space TEAM -v                 # List with verbose logging`,
	RunE: runListPages,
}

func runListPages(cmd *cobra.Command, args []string) error {
	if space == "" {
		return fmt.Errorf("space flag is required for list-pages command")
	}

	log := logger.New(verbose)

	cfg, err := config.LoadForListPages(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client := confluence.NewClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)

	pages, err := client.GetPageHierarchy(space, parentPage)
	if err != nil {
		return fmt.Errorf("failed to get page hierarchy: %w", err)
	}

	if parentPage != "" {
		fmt.Printf("Page hierarchy under '%s' in space '%s':\n\n", parentPage, space)
	} else {
		fmt.Printf("All pages in space '%s':\n\n", space)
	}

	printPageTree(pages, 0)
	return nil
}

func printPageTree(pages []confluence.PageInfo, indent int) {
	for _, page := range pages {
		prefix := ""
		for i := 0; i < indent; i++ {
			prefix += "  "
		}
		fmt.Printf("%s- %s (ID: %s)\n", prefix, page.Title, page.ID)
		if len(page.Children) > 0 {
			printPageTree(page.Children, indent+1)
		}
	}
}

func init() {
	rootCmd.AddCommand(listPagesCmd)

	// Local flags for list-pages command
	listPagesCmd.Flags().StringVarP(&space, "space", "s", "", "Confluence space key (required)")
	listPagesCmd.Flags().StringVarP(&parentPage, "parent", "p", "", "Parent page title to start from (optional)")

	// Mark space as required
	listPagesCmd.MarkFlagRequired("space")
}
