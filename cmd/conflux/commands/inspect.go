package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

var (
	inspectSpace  string
	inspectPage   string
	showChildren  bool
	showParents   bool
	showDetails   bool
)

// inspectCmd represents the inspect command
var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect page hierarchy and relationships in Confluence",
	Long: `Inspect page hierarchy and relationships in Confluence with detailed information.

This command helps debug page relationships and hierarchy issues by showing:
  - Page details (ID, title, parent information)
  - Children pages (if any)
  - Parent page chain (ancestors)
  - Page hierarchy visualization

You can specify a page by title or ID to start inspection from that page.`,
	Example: `  conflux inspect -space DOCS -page "My Page"        # Inspect by title
  conflux inspect -space DOCS -page "123456789"      # Inspect by ID  
  conflux inspect -space DOCS                       # Show space overview
  conflux inspect -space DOCS -page "Root" -details # Show detailed info`,
	RunE: runInspect,
}

func runInspect(cmd *cobra.Command, args []string) error {
	if inspectSpace == "" {
		return fmt.Errorf("space flag is required for inspect command")
	}

	log := logger.New(verbose)

	cfg, err := config.LoadForListPages(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	client := confluence.NewClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)

	// If no specific page requested, show space overview
	if inspectPage == "" {
		return inspectSpaceOverview(client, inspectSpace)
	}

	// Try to find the page by ID or title
	var targetPage *confluence.Page
	
	// Check if the input looks like a page ID (numeric)
	if isNumeric(inspectPage) {
		log.Debug("Attempting to find page by ID: %s", inspectPage)
		targetPage, err = client.GetPage(inspectPage)
		if err != nil {
			log.Debug("Failed to find page by ID, trying as title: %s", err)
		}
	}

	// If not found by ID or input is not numeric, try by title
	if targetPage == nil {
		log.Debug("Attempting to find page by title: %s", inspectPage)
		targetPage, err = client.FindPageByTitle(inspectSpace, inspectPage)
		if err != nil {
			return fmt.Errorf("failed to find page by title: %w", err)
		}
	}

	if targetPage == nil {
		return fmt.Errorf("page '%s' not found in space '%s'", inspectPage, inspectSpace)
	}

	return inspectPageDetails(client, targetPage, inspectSpace)
}

func inspectSpaceOverview(client confluence.ConfluenceClient, spaceKey string) error {
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
	
	// Show hierarchy
	printInspectPageTree(pages, 0, true)
	
	// Show summary
	totalPages := countTotalPages(pages)
	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("   🌳 Root pages: %d\n", len(pages))
	fmt.Printf("   📄 Total pages: %d\n", totalPages)
	
	return nil
}

func inspectPageDetails(client confluence.ConfluenceClient, page *confluence.Page, spaceKey string) error {
	fmt.Printf("🔍 Inspecting Page: %s\n", page.Title)
	fmt.Println(strings.Repeat("=", 50))

	// Basic page information
	fmt.Printf("📋 Page Details:\n")
	fmt.Printf("   🆔 ID: %s\n", page.ID)
	fmt.Printf("   📝 Title: %s\n", page.Title)
	fmt.Printf("   🏢 Space: %s\n", spaceKey)
	
	if showDetails {
		contentLength := len(page.Body.Storage.Value)
		fmt.Printf("   📊 Content Length: %d characters\n", contentLength)
		
		// Check for children macro
		hasChildrenMacro := strings.Contains(page.Body.Storage.Value, "ac:name=\"children\"")
		fmt.Printf("   🔗 Has Children Macro: %v\n", hasChildrenMacro)
		
		if hasChildrenMacro {
			fmt.Printf("   ℹ️  This appears to be a directory page\n")
		}
	}

	// Show parent chain (ancestors)
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

	// Show children
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

func printInspectPageTree(pages []confluence.PageInfo, indent int, isRoot bool) {
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

		// Print the page with icon and ID
		if isRoot {
			fmt.Printf("%s %s %s (ID: %s)\n", icon, prefix, page.Title, page.ID)
		} else {
			fmt.Printf("%s%s %s (ID: %s)\n", prefix, icon, page.Title, page.ID)
		}

		// Recursively print children
		if len(page.Children) > 0 {
			printInspectPageTree(page.Children, indent+1, false)
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
	rootCmd.AddCommand(inspectCmd)

	// Local flags for inspect command
	inspectCmd.Flags().StringVarP(&inspectSpace, "space", "s", "", "Confluence space key (required)")
	inspectCmd.Flags().StringVarP(&inspectPage, "page", "p", "", "Page title or ID to inspect (optional, shows space overview if omitted)")
	inspectCmd.Flags().BoolVar(&showChildren, "children", true, "Show children pages (default: true)")
	inspectCmd.Flags().BoolVarP(&showParents, "parents", "a", true, "Show parent chain/ancestors (default: true)")
	inspectCmd.Flags().BoolVarP(&showDetails, "details", "d", false, "Show detailed page information")

	// Mark space as required
	if err := inspectCmd.MarkFlagRequired("space"); err != nil {
		panic(fmt.Sprintf("Failed to mark space flag as required: %v", err))
	}
}