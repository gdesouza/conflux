package commands

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

// helper functions for processing Confluence HTML are provided by get_page.go
// to avoid duplication they were removed from pull.go and reused from get_page.go

var (
	pullSpace     string
	pullIDOrTitle string
	pullFormat    string
	pullProject   string
)

// pullCmd returns the raw page storage content for a page
var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Return the contents of a Confluence page",
	Long: `Fetch the storage-format content of a Confluence page by ID or title.

You must provide either:
  - a space key via --space, or
  - a project via --project (space inferred from project)

Then specify either a numeric page ID or a page title with --page.`,
	Example: `  conflux pull -space DOCS -page 123456789
  conflux pull -space DOCS -page "My Page Title"`,
	RunE: runPull,
}

func runPull(cmd *cobra.Command, args []string) error {
	if pullIDOrTitle == "" {
		return fmt.Errorf("page flag is required for pull command")
	}

	// Validate format
	switch pullFormat {
	case "", "storage", "html", "markdown":
		// ok (empty treated as storage)
	default:
		return fmt.Errorf("unsupported format: %s", pullFormat)
	}

	log := logger.New(verbose)

	cfg, err := config.LoadForListPages(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Project selection if provided
	if pullProject != "" {
		if err := cfg.SelectProject(pullProject); err != nil {
			return fmt.Errorf("failed to select project: %w", err)
		}
		if pullSpace == "" {
			pullSpace = cfg.Confluence.SpaceKey
		}
	}
	if pullSpace == "" {
		return fmt.Errorf("space flag or --project required for pull command")
	}

	client := newConfluenceClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)

	var page *confluence.Page

	// Try by ID if input looks numeric
	if isNumeric(pullIDOrTitle) {
		page, err = client.GetPage(pullIDOrTitle)
		if err != nil {
			log.Debug("failed to get page by ID: %v", err)
			page = nil
		}
	}

	// If not found by ID, try by title
	if page == nil {
		page, err = client.FindPageByTitle(pullSpace, pullIDOrTitle)
		if err != nil {
			return fmt.Errorf("failed to find page by title: %w", err)
		}
	}

	if page == nil {
		return fmt.Errorf("page '%s' not found in space '%s'", pullIDOrTitle, pullSpace)
	}

	// Print header then the requested format
	fmt.Printf("# %s (ID: %s)\n\n", page.Title, page.ID)

	format := pullFormat
	if format == "" {
		format = "storage"
	}

	content, err := generatePageOutput(page, format)
	if err != nil {
		return err
	}

	// Download attachments and update markdown
	attachments, err := client.ListAttachments(page.ID)
	if err != nil {
		log.Debug("failed to list attachments: %v", err)
		// Continue even if no attachments
	}

	attachmentDir := "attachments"
	if len(attachments) > 0 {
		if err := os.MkdirAll(attachmentDir, 0755); err != nil {
			return fmt.Errorf("failed to create attachment directory: %w", err)
		}
	}

	log.Debug("Found %d attachments for page %s", len(attachments), page.ID)
	for _, att := range attachments {
		log.Debug("Attachment: ID=%s Title=%s MediaType=%s Download=%s", att.ID, att.Title, att.MediaType, att.Links.Download)
		downloadURL, err := client.GetAttachmentDownloadURL(page.ID, att.ID)
		if err != nil {
			log.Debug("failed to get download URL for attachment %s: %v", att.Title, err)
			continue
		}
		localPath := fmt.Sprintf("%s/%s", attachmentDir, att.Title)
		req, err := http.NewRequest("GET", downloadURL, nil)
		if err != nil {
			log.Debug("Failed to create request for attachment %s: %v", att.Title, err)
			continue
		}
		req.SetBasicAuth(cfg.Confluence.Username, cfg.Confluence.APIToken)
		realClient, ok := client.(*confluence.Client)
		if !ok {
			log.Debug("Failed to access underlying HTTP client for attachment download")
			continue
		}
		resp, err := realClient.DoAuthenticatedRequest(req)
		if err != nil {
			log.Debug("Failed to download attachment %s: %v", att.Title, err)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			log.Debug("Failed to download attachment %s: status %d, body: %s", att.Title, resp.StatusCode, string(body))
			continue
		}
		log.Debug("Saving attachment to %s", localPath)
		f, err := os.Create(localPath)
		if err != nil {
			log.Debug("Failed to create local file for attachment %s: %v", att.Title, err)
			continue
		}
		_, err = io.Copy(f, resp.Body)
		f.Close()
		if err != nil {
			log.Debug("Failed to save attachment %s: %v", att.Title, err)
			continue
		}
		// Replace inline markdown references
		if strings.HasPrefix(att.MediaType, "image/") {
			// Image macro replacement now handles inline links; no further replacement needed
			// (If you need to support legacy or non-macro images, add logic here)
		} else {
			// Replace all file references
			content = strings.ReplaceAll(content, fmt.Sprintf("[%s]", att.Title), fmt.Sprintf("[%s](%s)", att.Title, localPath))
			content = strings.ReplaceAll(content, fmt.Sprintf("[]( %s)", att.Title), fmt.Sprintf("[%s](%s)", att.Title, localPath))
			content = strings.ReplaceAll(content, fmt.Sprintf("[]( %s)", localPath), fmt.Sprintf("[%s](%s)", att.Title, localPath))
		}
	}

	fmt.Println(content)
	return nil
}

// helper functions for processing Confluence HTML are provided by get_page.go
// to avoid duplication they were removed from pull.go and reused from get_page.go

// helper functions for processing Confluence HTML are provided by get_page.go
// to avoid duplication they were removed from pull.go and reused from get_page.go

func init() {
	rootCmd.AddCommand(pullCmd)

	pullCmd.Flags().StringVarP(&pullSpace, "space", "s", "", "Confluence space key (can be inferred from --project)")
	pullCmd.Flags().StringVarP(&pullIDOrTitle, "page", "p", "", "Page title or ID to fetch (required)")
	pullCmd.Flags().StringVarP(&pullFormat, "format", "f", "storage", "Output format: storage|html|markdown")
	pullCmd.Flags().StringVarP(&pullProject, "project", "P", "", "Project name defined in config to infer space")

	if err := pullCmd.MarkFlagRequired("page"); err != nil {
		panic(fmt.Sprintf("Failed to mark page flag as required: %v", err))
	}
}
