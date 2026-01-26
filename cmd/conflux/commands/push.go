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
	pushFile    string
	pushSpace   string
	pushParent  string
	pushProject string
)

// pushCmd pushes (creates or updates) a single markdown file as a Confluence page
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push a single markdown file to Confluence",
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
	RunE: runPush,
}

func runPush(cmd *cobra.Command, args []string) error {
	if pushFile == "" {
		return fmt.Errorf("file flag is required for push command")
	}

	info, err := os.Stat(pushFile)
	if err != nil {
		return fmt.Errorf("failed to access file: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("%s is a directory; provide a single markdown file", pushFile)
	}
	if strings.ToLower(filepath.Ext(pushFile)) != ".md" {
		return fmt.Errorf("file must have .md extension: %s", pushFile)
	}

	log := logger.New(verbose)

	// Load relaxed config similar to list-pages (space can be provided by flags / project)
	cfg, err := config.LoadForListPages(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Project selection if provided
	if pushProject != "" {
		if err := cfg.SelectProject(pushProject); err != nil {
			return fmt.Errorf("failed to select project: %w", err)
		}
		if pushSpace == "" {
			pushSpace = cfg.Confluence.SpaceKey
		}
	} else if pushSpace == "" && cfg.Confluence.SpaceKey == "" && len(cfg.Projects) > 0 {
		// Apply default project if nothing specified
		cfg.ApplyDefaultProject()
		pushSpace = cfg.Confluence.SpaceKey
	}

	if pushSpace == "" {
		return fmt.Errorf("space flag or --project required for push command")
	}

	client := newConfluenceClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)

	// Parse markdown file
	doc, err := markdown.ParseFile(pushFile)
	if err != nil {
		return fmt.Errorf("failed to parse markdown file: %w", err)
	}
	log.Debug("Parsed markdown file: title=%s", doc.Title)

	// Convert markdown -> Confluence storage format (initial pass without attachments/mermaid images)
	content := markdown.ConvertToConfluenceFormat(doc.Content)

	// Resolve parent ID if provided
	var parentID string
	if pushParent != "" {
		if isNumeric(pushParent) { // treat as ID
			parentID = pushParent
			log.Debug("Using numeric parent page ID: %s", parentID)
		} else {
			log.Debug("Resolving parent by title: %s", pushParent)
			parentPage, err := client.FindPageByTitle(pushSpace, pushParent)
			if err != nil {
				return fmt.Errorf("failed to resolve parent page '%s': %w", pushParent, err)
			}
			if parentPage == nil {
				return fmt.Errorf("parent page '%s' not found in space '%s'", pushParent, pushSpace)
			}
			parentID = parentPage.ID
		}
	}

	// Determine if page exists already (lookup by title)
	existing, err := client.FindPageByTitle(pushSpace, doc.Title)
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
		fmt.Printf("Updated page '%s' (ID: %s) in space '%s'\n", page.Title, page.ID, pushSpace)
	} else {
		if parentID != "" {
			log.Debug("Creating new page with parent %s", parentID)
			page, err = client.CreatePageWithParent(pushSpace, doc.Title, content, parentID)
		} else {
			log.Debug("Creating new root page in space %s", pushSpace)
			page, err = client.CreatePage(pushSpace, doc.Title, content)
		}
		if err != nil {
			return fmt.Errorf("failed to create page: %w", err)
		}
		fmt.Printf("Created page '%s' (ID: %s) in space '%s'\n", page.Title, page.ID, pushSpace)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(pushCmd)

	pushCmd.Flags().StringVarP(&pushFile, "file", "f", "", "Path to local markdown file (required)")
	pushCmd.Flags().StringVarP(&pushSpace, "space", "s", "", "Confluence space key (can be inferred from --project)")
	pushCmd.Flags().StringVarP(&pushParent, "parent", "p", "", "Optional parent page title or ID")
	pushCmd.Flags().StringVarP(&pushProject, "project", "P", "", "Project name defined in config to infer space")

	if err := pushCmd.MarkFlagRequired("file"); err != nil {
		panic(fmt.Sprintf("Failed to mark file flag as required: %v", err))
	}
}
