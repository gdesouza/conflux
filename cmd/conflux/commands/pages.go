package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

var (
	pagesSpace   string
	pagesParent  string
	pagesProject string

	pagesShowPage    string
	pagesShowDetails bool
)

var pagesCmd = &cobra.Command{
	Use:   "pages",
	Short: "List and inspect Confluence pages",
	Long: `List page hierarchy from a Confluence space with visual tree formatting.

This command connects to Confluence and retrieves the page hierarchy for a specified
space, displaying it with icons and tree formatting for easy navigation:
  🏢 Space indicators
  📁 Folders (pages with children)
  📄 Pages (leaf nodes)

You can optionally specify a parent page to start the hierarchy from.`,
	Example: `  conflux pages -s DOCS                     # List all pages in space
  conflux pages -s DOCS -p "API"             # List pages under parent
  conflux pages -P myproject                 # List pages for project
  conflux pages show -s DOCS -p "My Page"    # Show detailed page info`,
	RunE: runPages,
}

var pagesShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show detailed page information",
	Long: `Inspect page hierarchy and relationships in Confluence with detailed information.

This command helps debug page relationships and hierarchy issues by showing:
  - Page details (ID, title, parent information)
  - Children pages (if any)
  - Parent page chain (ancestors)
  - Page hierarchy visualization

Space may be selected through --space, --project, CONFLUX_SPACE_KEY,
confluence.space_key, or the first configured project. You can specify a page by title or ID to start inspection from that page; if omitted an overview of the space roots is shown.`,
	Example: `  conflux pages show -s DOCS -p "My Page"        # Inspect by title
  conflux pages show -s DOCS -p "123456789"      # Inspect by ID  
  conflux pages show -s DOCS                     # Show space overview
  conflux pages show -s DOCS -p "Root" -d        # Show detailed info`,
	RunE: runPagesShow,
}

func runPages(cmd *cobra.Command, args []string) error {
	log := logger.New(verbose)

	runtime, err := resolveRuntimeConfig(pagesSpace, pagesProject)
	if err != nil {
		return err
	}
	effectiveSpace := runtime.Confluence.SpaceKey

	client := newConfluenceClient(runtime.Confluence.BaseURL, runtime.Confluence.Username, runtime.Confluence.APIToken, log)

	pages, err := client.GetPageHierarchy(effectiveSpace, pagesParent)
	if err != nil {
		return fmt.Errorf("failed to get page hierarchy: %w", err)
	}

	if pagesParent != "" {
		fmt.Printf("🏢 Space '%s' → 📁 '%s':\n\n", effectiveSpace, pagesParent)
	} else {
		fmt.Printf("🏢 Space '%s':\n\n", effectiveSpace)
	}

	printPageTree(pages, 0, true)
	return nil
}

func runPagesShow(cmd *cobra.Command, args []string) error {
	log := logger.New(verbose)

	runtime, err := resolveRuntimeConfig(pagesSpace, pagesProject)
	if err != nil {
		return err
	}
	effectiveSpace := runtime.Confluence.SpaceKey

	client := newConfluenceClient(runtime.Confluence.BaseURL, runtime.Confluence.Username, runtime.Confluence.APIToken, log)

	if pagesShowPage == "" {
		return showSpaceOverview(client, effectiveSpace)
	}

	var targetPage *confluence.Page

	if isNumeric(pagesShowPage) {
		log.Debug("Attempting to find page by ID: %s", pagesShowPage)
		targetPage, err = client.GetPage(pagesShowPage)
		if err != nil {
			log.Debug("Failed to find page by ID, trying as title: %s", err)
		}
	}

	if targetPage == nil {
		log.Debug("Attempting to find page by title: %s", pagesShowPage)
		targetPage, err = client.FindPageByTitle(effectiveSpace, pagesShowPage)
		if err != nil {
			return fmt.Errorf("failed to find page by title: %w", err)
		}
	}

	if targetPage == nil {
		return fmt.Errorf("page '%s' not found in space '%s'", pagesShowPage, effectiveSpace)
	}

	return showPageDetails(client, targetPage, effectiveSpace)
}

func showSpaceOverview(client confluence.ConfluenceClient, spaceKey string) error {
	fmt.Printf("🏢 Inspecting Space: %s\n", spaceKey)
	fmt.Println(strings.Repeat("=", 50))

	pages, err := client.GetPageHierarchy(spaceKey, "")
	if err != nil {
		return fmt.Errorf("failed to get page hierarchy: %w", err)
	}

	if len(pages) == 0 {
		fmt.Println("📭 No pages found in this space")
		return nil
	}

	fmt.Printf("📊 Found %d root pages in space\n\n", len(pages))

	printPageTree(pages, 0, true)

	totalPages := countTotalPages(pages)
	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("   🌳 Root pages: %d\n", len(pages))
	fmt.Printf("   📄 Total pages: %d\n", totalPages)

	return nil
}

func showPageDetails(client confluence.ConfluenceClient, page *confluence.Page, spaceKey string) error {
	fmt.Printf("🔍 Inspecting Page: %s\n", page.Title)
	fmt.Println(strings.Repeat("=", 50))

	fmt.Printf("📋 Page Details:\n")
	fmt.Printf("   🆔 ID: %s\n", page.ID)
	fmt.Printf("   📝 Title: %s\n", page.Title)
	fmt.Printf("   🏢 Space: %s\n", spaceKey)

	if pagesShowDetails {
		contentLength := len(page.Body.Storage.Value)
		fmt.Printf("   📊 Content Length: %d characters\n", contentLength)

		hasChildrenMacro := strings.Contains(page.Body.Storage.Value, "ac:name=\"children\"")
		fmt.Printf("   🔗 Has Children Macro: %v\n", hasChildrenMacro)

		if hasChildrenMacro {
			fmt.Printf("   ℹ️  This appears to be a directory page\n")
		}
	}

	fmt.Printf("\n👆 Parent Chain:\n")
	ancestors, err := client.GetPageAncestors(page.ID)
	if err != nil {
		fmt.Printf("   ❌ Failed to get ancestors: %s\n", err)
	} else if len(ancestors) == 0 {
		fmt.Printf("   🏠 This is a root page (no parents)\n")
	} else {
		for i, ancestor := range ancestors {
			fmt.Printf("   %s 📁 %s (ID: %s)\n",
				strings.Repeat("  ", i), ancestor.Title, ancestor.ID)
		}
		fmt.Printf("   %s 📄 %s (ID: %s) ← Current Page\n",
			strings.Repeat("  ", len(ancestors)), page.Title, page.ID)
	}

	fmt.Printf("\n👇 Children:\n")
	children, err := client.GetChildPages(page.ID)
	if err != nil {
		fmt.Printf("   ❌ Failed to get children: %s\n", err)
	} else if len(children) == 0 {
		fmt.Printf("   📭 No child pages\n")
	} else {
		fmt.Printf("   📊 Found %d child pages:\n", len(children))
		for _, child := range children {
			fmt.Printf("     📄 %s (ID: %s)\n", child.Title, child.ID)
		}
	}

	return nil
}

func printPageTree(pages []confluence.PageInfo, indent int, isRoot bool) {
	for i, page := range pages {
		isLast := i == len(pages)-1

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

		var icon string
		if len(page.Children) > 0 {
			icon = "📁"
		} else {
			icon = "📄"
		}

		if isRoot {
			fmt.Printf("%s %s %s (ID: %s)\n", icon, prefix, page.Title, page.ID)
		} else {
			fmt.Printf("%s%s %s (ID: %s)\n", prefix, icon, page.Title, page.ID)
		}

		if len(page.Children) > 0 {
			printPageTree(page.Children, indent+1, false)
		}
	}
}

func countTotalPages(pages []confluence.PageInfo) int {
	total := len(pages)
	for _, page := range pages {
		total += countTotalPages(page.Children)
	}
	return total
}

func isNumeric(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func init() {
	rootCmd.AddCommand(pagesCmd)
	pagesCmd.AddCommand(pagesShowCmd)

	pagesCmd.PersistentFlags().StringVarP(&pagesSpace, "space", "s", "", "Confluence space key (can be inferred from --project)")
	pagesCmd.PersistentFlags().StringVarP(&pagesProject, "project", "P", "", "Project name defined in config to infer space")

	pagesCmd.Flags().StringVarP(&pagesParent, "parent", "p", "", "Parent page title to start from (optional)")

	pagesShowCmd.Flags().StringVarP(&pagesShowPage, "page", "p", "", "Page title or ID to inspect (optional, shows space overview if omitted)")
	pagesShowCmd.Flags().BoolVarP(&pagesShowDetails, "details", "d", false, "Show detailed page information")
}
