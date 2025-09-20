package main

import (
	"flag"
	"fmt"

	"conflux/internal/config"
	"conflux/internal/sync"
	"conflux/pkg/logger"
)

var (
	configFile = flag.String("config", "config.yaml", "path to configuration file")
	verbose    = flag.Bool("verbose", false, "enable verbose logging")
	dryRun     = flag.Bool("dry-run", false, "perform a dry run without making changes")
	help       = flag.Bool("help", false, "show help message")
)

func main() {
	flag.Parse()

	if *help {
		printUsage()
		return
	}

	log := logger.New(*verbose)

	cfg, err := config.Load(*configFile)
	if err != nil {
		log.Fatal("Failed to load config: %v", err)
	}

	syncer := sync.New(cfg, log)

	if err := syncer.Sync(*dryRun); err != nil {
		log.Fatal("Sync failed: %v", err)
	}

	fmt.Println("Sync completed successfully!")
}

func printUsage() {
	fmt.Printf("Conflux - Sync local markdown files to Confluence\n\n")
	fmt.Printf("USAGE:\n")
	fmt.Printf("    conflux [FLAGS]\n\n")
	fmt.Printf("FLAGS:\n")
	flag.PrintDefaults()
	fmt.Printf("\nEXAMPLES:\n")
	fmt.Printf("    conflux -config my-config.yaml\n")
	fmt.Printf("    conflux -dry-run -verbose\n")
	fmt.Printf("    conflux -help\n\n")
	fmt.Printf("For more information, see: https://github.com/your-org/conflux\n")
}
