package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/internal/markdown"
	"conflux/pkg/logger"
)

var (
	uploadFile    string
	uploadSpace   string
	uploadParent  string
	uploadProject string
)

// uploadCmd uploads (creates or updates) a single markdown file as a Confluence page
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload a single markdown file to Confluence",
	Long: `Create or update a Confluence page from a single local markdown file.

Space resolution precedence:
  1. --space flag
  2. --project flag (project's space)
  3. First project in config (implicit default, if space unset)
  4. Top-level confluence.space_key (legacy single-project)

Parent resolution:
  - If --parent looks numeric it is treated as a page ID
  - Otherwise it is resolved as a title in the target space.

If a page with the markdown title already exists it will be updated; otherwise it will be created.`,
	RunE: runUpload,
}

func runUpload(cmd *cobra.Command, args []string) error {
	if uploadFile == "" {
		return fmt.Errorf("file flag is required for upload command")
	}

	info, err := os.Stat(uploadFile)
	if err != nil {
		return fmt.Errorf("failed to access file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory; provide a single markdown file", uploadFile)
	}
	if strings.ToLower(filepath.Ext(uploadFile)) != ".md" {
		return fmt.Errorf("file must have .md extension: %s", uploadFile)
	}

	log := logger.New(verbose)

	// Load relaxed config similar to list-pages (space can be provided by flags / project)
	cfg, err := config.LoadForListPages(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Project selection if provided
	if uploadProject != "" {
		if err := cfg.SelectProject(uploadProject); err != nil {
			return fmt.Errorf("failed to select project: %w", err)
		}
		if uploadSpace == "" {
			uploadSpace = cfg.Confluence.SpaceKey
		}
	} else if uploadSpace == "" && cfg.Confluence.SpaceKey == "" && len(cfg.Projects) > 0 {
		// Apply default project if nothing specified
		cfg.ApplyDefaultProject()
		uploadSpace = cfg.Confluence.SpaceKey
	}

	if uploadSpace == "" {
		return fmt.Errorf("space flag or --project required for upload command")
	}

	client := newConfluenceClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)

	// Parse markdown file
	doc, err := markdown.ParseFile(uploadFile)
	if err != nil {
		return fmt.Errorf("failed to parse markdown file: %w", err)
	}
	log.Debug("Parsed markdown file: title=%s", doc.Title)

	// Convert markdown -> Confluence storage format (initial pass without attachments/mermaid images)
	content := markdown.ConvertToConfluenceFormat(doc.Content)

	// Resolve parent ID if provided
	var parentID string
	if uploadParent != "" {
		if isNumeric(uploadParent) { // treat as ID
			parentID = uploadParent
			log.Debug("Using numeric parent page ID: %s", parentID)
		} else {
			log.Debug("Resolving parent by title: %s", uploadParent)
			parentPage, err := client.FindPageByTitle(uploadSpace, uploadParent)
			if err != nil {
				return fmt.Errorf("failed to resolve parent page '%s': %w", uploadParent, err)
			}
			if parentPage == nil {
				return fmt.Errorf("parent page '%s' not found in space '%s'", uploadParent, uploadSpace)
			}
			parentID = parentPage.ID
		}
	}

	// Determine if page exists already (lookup by title)
	existing, err := client.FindPageByTitle(uploadSpace, doc.Title)
	if err != nil {
		return fmt.Errorf("failed to search for existing page: %w", err)
	}

	var page *confluence.Page
	if existing != nil {
		log.Debug("Updating existing page ID=%s title=%s", existing.ID, existing.Title)
		page, err = client.UpdatePage(existing.ID, doc.Title, content)
		if err != nil {
			return fmt.Errorf("failed to update page: %w", err)
		}
		fmt.Printf("Updated page '%s' (ID: %s) in space '%s'\n", page.Title, page.ID, uploadSpace)
	} else {
		if parentID != "" {
			log.Debug("Creating new page with parent %s", parentID)
			page, err = client.CreatePageWithParent(uploadSpace, doc.Title, content, parentID)
		} else {
			log.Debug("Creating new root page in space %s", uploadSpace)
			page, err = client.CreatePage(uploadSpace, doc.Title, content)
		}
		if err != nil {
			return fmt.Errorf("failed to create page: %w", err)
		}
		fmt.Printf("Created page '%s' (ID: %s) in space '%s'\n", page.Title, page.ID, uploadSpace)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(uploadCmd)

	uploadCmd.Flags().StringVarP(&uploadFile, "file", "f", "", "Path to local markdown file (required)")
	uploadCmd.Flags().StringVarP(&uploadSpace, "space", "s", "", "Confluence space key (can be inferred from --project)")
	uploadCmd.Flags().StringVarP(&uploadParent, "parent", "p", "", "Optional parent page title or ID")
	uploadCmd.Flags().StringVarP(&uploadProject, "project", "P", "", "Project name defined in config to infer space")

	if err := uploadCmd.MarkFlagRequired("file"); err != nil {
		panic(fmt.Sprintf("Failed to mark file flag as required: %v", err))
	}
}
