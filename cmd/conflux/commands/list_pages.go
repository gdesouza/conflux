package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

var (
	space       string
	parentPage  string
	listProject string
)

// listPagesCmd represents the list-pages command
var listPagesCmd = &cobra.Command{
	Use:   "list-pages",
	Short: "List page hierarchy from a Confluence space",
	Long: `List page hierarchy from a Confluence space with visual tree formatting.

This command connects to Confluence and retrieves the page hierarchy for a specified
space, displaying it with icons and tree formatting for easy navigation:
  🏢 Space indicators
  📁 Folders (pages with children)
  📄 Pages (leaf nodes)

You can optionally specify a parent page to start the hierarchy from.`,
	Example: `  conflux list-pages -space DOCS                     # List all pages in space
  conflux list-pages -space DOCS -parent "API"      # List pages under parent
  conflux list-pages -space TEAM -v                 # List with verbose logging`,
	RunE: runListPages,
}

func runListPages(cmd *cobra.Command, args []string) error {
	log := logger.New(verbose)

	cfg, err := config.LoadForListPages(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Project selection if provided
	if listProject != "" {
		if err := cfg.SelectProject(listProject); err != nil {
			return fmt.Errorf("failed to select project: %w", err)
		}
		if space == "" {
			space = cfg.Confluence.SpaceKey
		}
	}

	if space == "" {
		return fmt.Errorf("space flag or --project required for list-pages command")
	}

	client := confluence.NewClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)

	pages, err := client.GetPageHierarchy(space, parentPage)
	if err != nil {
		return fmt.Errorf("failed to get page hierarchy: %w", err)
	}

	if parentPage != "" {
		fmt.Printf("🏢 Space '%s' → 📁 '%s':\n\n", space, parentPage)
	} else {
		fmt.Printf("🏢 Space '%s':\n\n", space)
	}

	printPageTree(pages, 0, true)
	return nil
}

func printPageTree(pages []confluence.PageInfo, indent int, isRoot bool) {
	for i, page := range pages {
		isLast := i == len(pages)-1

		// Build prefix with proper tree formatting
		prefix := ""
		if !isRoot {
			for j := 0; j < indent; j++ {
				prefix += "  "
			}
			if isLast {
				prefix += "└── "
			} else {
				prefix += "├── "
			}
		}

		// Choose icon based on whether page has children
		var icon string
		if len(page.Children) > 0 {
			icon = "📁"
		} else {
			icon = "📄"
		}

		// Print the page with icon
		if isRoot {
			fmt.Printf("%s %s %s (ID: %s)\n", icon, prefix, page.Title, page.ID)
		} else {
			fmt.Printf("%s%s %s (ID: %s)\n", prefix, icon, page.Title, page.ID)
		}

		// Recursively print children
		if len(page.Children) > 0 {
			printPageTree(page.Children, indent+1, false)
		}
	}
}

func init() {
	rootCmd.AddCommand(listPagesCmd)

	// Local flags for list-pages command
	listPagesCmd.Flags().StringVarP(&space, "space", "s", "", "Confluence space key (can be inferred from --project)")
	listPagesCmd.Flags().StringVarP(&parentPage, "parent", "p", "", "Parent page title to start from (optional)")
	listPagesCmd.Flags().StringVarP(&listProject, "project", "P", "", "Project name defined in config to infer space")
}
