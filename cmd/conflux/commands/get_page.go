package commands

import (
	"fmt"

	htmldoc "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

var (
	getPageSpace     string
	getPageIDOrTitle string
	getPageFormat    string
	getPageProject   string
)

// getPageCmd returns the raw page storage content for a page
var getPageCmd = &cobra.Command{
	Use:   "get-page",
	Short: "Return the contents of a Confluence page",
	Long: `Fetch the storage-format content of a Confluence page by ID or title.

You must provide either:
  - a space key via --space, or
  - a project via --project (space inferred from project)

Then specify either a numeric page ID or a page title with --page.`,
	Example: `  conflux get-page -space DOCS -page 123456789
  conflux get-page -space DOCS -page "My Page Title"`,
	RunE: runGetPage,
}

func runGetPage(cmd *cobra.Command, args []string) error {
	if getPageIDOrTitle == "" {
		return fmt.Errorf("page flag is required for get-page command")
	}

	// Validate format
	switch getPageFormat {
	case "", "storage", "html", "markdown":
		// ok (empty treated as storage)
	default:
		return fmt.Errorf("unsupported format: %s", getPageFormat)
	}

	log := logger.New(verbose)

	cfg, err := config.LoadForListPages(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Project selection if provided
	if getPageProject != "" {
		if err := cfg.SelectProject(getPageProject); err != nil {
			return fmt.Errorf("failed to select project: %w", err)
		}
		if getPageSpace == "" {
			getPageSpace = cfg.Confluence.SpaceKey
		}
	}
	if getPageSpace == "" {
		return fmt.Errorf("space flag or --project required for get-page command")
	}

	client := confluence.NewClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)

	var page *confluence.Page

	// Try by ID if input looks numeric
	if isNumeric(getPageIDOrTitle) {
		page, err = client.GetPage(getPageIDOrTitle)
		if err != nil {
			log.Debug("failed to get page by ID: %v", err)
			page = nil
		}
	}

	// If not found by ID, try by title
	if page == nil {
		page, err = client.FindPageByTitle(getPageSpace, getPageIDOrTitle)
		if err != nil {
			return fmt.Errorf("failed to find page by title: %w", err)
		}
	}

	if page == nil {
		return fmt.Errorf("page '%s' not found in space '%s'", getPageIDOrTitle, getPageSpace)
	}

	// Print header then the requested format
	fmt.Printf("# %s (ID: %s)\n\n", page.Title, page.ID)

	format := getPageFormat
	if format == "" {
		format = "storage"
	}

	content, err := generatePageOutput(page, format)
	if err != nil {
		return err
	}
	fmt.Println(content)
	return nil
}

// generatePageOutput returns the page content in the requested format.
// It does not include the header line with title/ID.
func generatePageOutput(page *confluence.Page, format string) (string, error) {
	switch format {
	case "storage":
		return page.Body.Storage.Value, nil
	case "html":
		if page.Body.View.Value != "" {
			return page.Body.View.Value, nil
		}
		return page.Body.Storage.Value, nil
	case "markdown":
		var html string
		if page.Body.View.Value != "" {
			html = page.Body.View.Value
		} else {
			html = page.Body.Storage.Value
		}
		md, err := htmldoc.ConvertString(html)
		if err != nil {
			return html, nil // fallback to raw HTML on conversion errors
		}
		return string(md), nil
	default:
		return "", fmt.Errorf("unsupported format: %s", format)
	}
}

func init() {
	rootCmd.AddCommand(getPageCmd)

	getPageCmd.Flags().StringVarP(&getPageSpace, "space", "s", "", "Confluence space key (can be inferred from --project)")
	getPageCmd.Flags().StringVarP(&getPageIDOrTitle, "page", "p", "", "Page title or ID to fetch (required)")
	getPageCmd.Flags().StringVarP(&getPageFormat, "format", "f", "storage", "Output format: storage|html|markdown")
	getPageCmd.Flags().StringVarP(&getPageProject, "project", "P", "", "Project name defined in config to infer space")

	if err := getPageCmd.MarkFlagRequired("page"); err != nil {
		panic(fmt.Sprintf("Failed to mark page flag as required: %v", err))
	}
}
