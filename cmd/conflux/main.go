package main

import (
	"flag"
	"fmt"
	"os"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/internal/sync"
	"conflux/pkg/logger"
)

func main() {
	if len(os.Args) < 2 {
		// Default behavior: sync command
		runSync(os.Args[1:])
		return
	}

	command := os.Args[1]
	switch command {
	case "sync":
		runSync(os.Args[2:])
	case "list-pages":
		runListPages(os.Args[2:])
	case "-help", "--help", "help":
		printUsage()
	default:
		// Check if first argument is a flag, then it's the default sync command
		if command[0] == '-' {
			runSync(os.Args[1:])
		} else {
			fmt.Printf("Unknown command: %s\n\n", command)
			printUsage()
			os.Exit(1)
		}
	}
}

func runSync(args []string) {
	syncCmd := flag.NewFlagSet("sync", flag.ExitOnError)
	configFile := syncCmd.String("config", "config.yaml", "path to configuration file")
	verbose := syncCmd.Bool("verbose", false, "enable verbose logging")
	dryRun := syncCmd.Bool("dry-run", false, "perform a dry run without making changes")
	docsDir := syncCmd.String("docs", ".", "path to local markdown documents directory")
	help := syncCmd.Bool("help", false, "show help message")

	syncCmd.Parse(args)

	if *help {
		printSyncUsage()
		return
	}

	log := logger.New(*verbose)

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatal("Failed to load config: %v", err)
	}

	// Override config markdown directory with CLI flag
	cfg.Local.MarkdownDir = *docsDir

	syncer := sync.New(cfg, log)

	if err := syncer.Sync(*dryRun); err != nil {
		log.Fatal("Sync failed: %v", err)
	}

	fmt.Println("Sync completed successfully!")
}

func runListPages(args []string) {
	listCmd := flag.NewFlagSet("list-pages", flag.ExitOnError)
	configFile := listCmd.String("config", "config.yaml", "path to configuration file")
	verbose := listCmd.Bool("verbose", false, "enable verbose logging")
	space := listCmd.String("space", "", "Confluence space key")
	parentPage := listCmd.String("parent", "", "Parent page title (optional)")
	help := listCmd.Bool("help", false, "show help message")

	listCmd.Parse(args)

	if *help {
		printListPagesUsage()
		return
	}

	if *space == "" {
		fmt.Println("Error: -space flag is required for list-pages command")
		fmt.Println()
		printListPagesUsage()
		os.Exit(1)
	}

	log := logger.New(*verbose)

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatal("Failed to load config: %v", err)
	}

	client := confluence.NewClient(cfg.Confluence.BaseURL, cfg.Confluence.Username, cfg.Confluence.APIToken, log)

	pages, err := client.GetPageHierarchy(*space, *parentPage)
	if err != nil {
		log.Fatal("Failed to get page hierarchy: %v", err)
	}

	if *parentPage != "" {
		fmt.Printf("Page hierarchy under '%s' in space '%s':\n\n", *parentPage, *space)
	} else {
		fmt.Printf("All pages in space '%s':\n\n", *space)
	}

	printPageTree(pages, 0)
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

func printUsage() {
	fmt.Printf("Conflux - Sync local markdown files to Confluence\n\n")
	fmt.Printf("USAGE:\n")
	fmt.Printf("    conflux [COMMAND] [FLAGS]\n\n")
	fmt.Printf("COMMANDS:\n")
	fmt.Printf("    sync         Sync local markdown files to Confluence (default)\n")
	fmt.Printf("    list-pages   List page hierarchy from a Confluence space\n")
	fmt.Printf("    help         Show this help message\n\n")
	fmt.Printf("Use 'conflux [COMMAND] -help' for more information about a command.\n\n")
	fmt.Printf("EXAMPLES:\n")
	fmt.Printf("    conflux                                    # Sync current directory\n")
	fmt.Printf("    conflux sync -docs ./docs -dry-run        # Sync with options\n")
	fmt.Printf("    conflux list-pages -space DOCS            # List all pages\n")
	fmt.Printf("    conflux list-pages -space DOCS -parent \"API\"  # List under parent\n\n")
	fmt.Printf("For more information, see: https://github.com/your-org/conflux\n")
}

func printSyncUsage() {
	fmt.Printf("Sync local markdown files to Confluence\n\n")
	fmt.Printf("USAGE:\n")
	fmt.Printf("    conflux sync [FLAGS]\n\n")
	fmt.Printf("FLAGS:\n")
	fmt.Printf("    -config      Path to configuration file (default: config.yaml)\n")
	fmt.Printf("    -verbose     Enable verbose logging\n")
	fmt.Printf("    -docs        Path to markdown documents directory (default: .)\n")
	fmt.Printf("    -dry-run     Perform a dry run without making changes\n")
	fmt.Printf("    -help        Show this help message\n\n")
	fmt.Printf("EXAMPLES:\n")
	fmt.Printf("    conflux sync\n")
	fmt.Printf("    conflux sync -docs ./documentation -dry-run\n")
	fmt.Printf("    conflux sync -config prod-config.yaml -verbose\n")
}

func printListPagesUsage() {
	fmt.Printf("List page hierarchy from a Confluence space\n\n")
	fmt.Printf("USAGE:\n")
	fmt.Printf("    conflux list-pages -space SPACEKEY [FLAGS]\n\n")
	fmt.Printf("FLAGS:\n")
	fmt.Printf("    -config      Path to configuration file (default: config.yaml)\n")
	fmt.Printf("    -verbose     Enable verbose logging\n")
	fmt.Printf("    -space       Confluence space key (required)\n")
	fmt.Printf("    -parent      Parent page title to start from (optional)\n")
	fmt.Printf("    -help        Show this help message\n\n")
	fmt.Printf("EXAMPLES:\n")
	fmt.Printf("    conflux list-pages -space DOCS\n")
	fmt.Printf("    conflux list-pages -space DOCS -parent \"API Documentation\"\n")
	fmt.Printf("    conflux list-pages -space TEAM -verbose\n")
}
