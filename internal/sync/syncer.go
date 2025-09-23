package sync

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/internal/markdown"
	"conflux/internal/mermaid"
	"conflux/pkg/logger"
)

type SyncStatus string

const (
	StatusNew      SyncStatus = "new"
	StatusChanged  SyncStatus = "changed"
	StatusUpToDate SyncStatus = "up-to-date"
)

type PageSyncInfo struct {
	Title       string
	FilePath    string
	Status      SyncStatus
	Level       int
	Children    []PageSyncInfo
	ParentPath  string // Directory path that should be the parent page
	IsDirectory bool   // True if this represents a directory page
}

type Syncer struct {
	config     *config.Config
	confluence confluence.ConfluenceClient
	logger     *logger.Logger
	metadata   *SyncMetadata
}

func New(cfg *config.Config, log *logger.Logger) *Syncer {
	client := confluence.NewClient(
		cfg.Confluence.BaseURL,
		cfg.Confluence.Username,
		cfg.Confluence.APIToken,
		log,
	)

	metadata := NewSyncMetadata(cfg.Local.MarkdownDir, cfg.Confluence.SpaceKey)

	return &Syncer{
		config:     cfg,
		confluence: client,
		logger:     log,
		metadata:   metadata,
	}
}

// NewWithClient creates a syncer with a custom confluence client (useful for testing)
func NewWithClient(cfg *config.Config, log *logger.Logger, client confluence.ConfluenceClient) *Syncer {
	metadata := NewSyncMetadata(cfg.Local.MarkdownDir, cfg.Confluence.SpaceKey)

	return &Syncer{
		config:     cfg,
		confluence: client,
		logger:     log,
		metadata:   metadata,
	}
}

func (s *Syncer) Sync(dryRun bool, force bool) error {
	return s.SyncWithFile(dryRun, force, "")
}

func (s *Syncer) SyncWithFile(dryRun bool, force bool, singleFilePath string) error {
	s.logger.Info("Starting sync process...")

	// Load sync metadata cache
	if err := s.metadata.Load(); err != nil {
		s.logger.Debug("Could not load sync cache (will create new): %v", err)
	}

	// Check mermaid dependencies if mermaid mode requires CLI
	if s.config.Mermaid.Mode == "convert-to-image" {
		processor := mermaid.NewProcessor(&s.config.Mermaid, s.logger)
		if err := processor.CheckDependencies(); err != nil {
			s.logger.Error("Mermaid CLI dependencies not met: %v", err)
			s.logger.Info("Install mermaid CLI: npm install -g @mermaid-js/mermaid-cli")
			return fmt.Errorf("mermaid dependencies not available: %w", err)
		}
		s.logger.Info("Mermaid CLI dependencies verified")
	}

	var files []string
	var err error

	if singleFilePath != "" {
		// Single file mode: use FindMarkdownFiles for validation
		files, err = markdown.FindMarkdownFiles(singleFilePath, []string{})
		if err != nil {
			return fmt.Errorf("failed to process file %s: %w", singleFilePath, err)
		}
		s.logger.Info("Single file mode: processing %s", singleFilePath)
	} else {
		// Directory mode: find all markdown files
		files, err = markdown.FindMarkdownFiles(s.config.Local.MarkdownDir, s.config.Local.Exclude)
		if err != nil {
			return fmt.Errorf("failed to find markdown files: %w", err)
		}
		s.logger.Info("Found %d markdown files to analyze", len(files))
	}

	// Analyze all files and build page hierarchy
	pages, err := s.analyzeAllFiles(files)
	if err != nil {
		return fmt.Errorf("failed to analyze files: %w", err)
	}

	// Count status types
	var newCount, changedCount, upToDateCount int
	for _, page := range pages {
		switch page.Status {
		case StatusNew:
			newCount++
		case StatusChanged:
			changedCount++
		case StatusUpToDate:
			upToDateCount++
		}
	}

	// Display hierarchy and get user confirmation (unless it's a dry run or force mode)
	if dryRun {
		s.displayEnhancedHierarchy(pages, newCount, changedCount, upToDateCount, true)
		return nil
	}

	s.displayEnhancedHierarchy(pages, newCount, changedCount, upToDateCount, false)

	if !force {
		choice := PromptForConfirmation("Sync Preview", changedCount, newCount, upToDateCount)
		switch choice.Action {
		case "cancel":
			fmt.Println("❌ Sync canceled by user")
			return nil
		case "select":
			// Display file list for selection
			_ = DisplayFileList(pages, true)
			selectionChoice := PromptForFileSelection()
			if selectionChoice.Action == "cancel" {
				fmt.Println("❌ Sync canceled by user")
				return nil
			}
			// TODO: Implement selective sync based on user selection
			// For now, proceed with all files
		case "continue":
			// Proceed with sync
		}
	}

	// Perform the actual sync
	return s.performEnhancedSync(pages)
}

func (s *Syncer) extractDirectories(files []string) []string {
	dirSet := make(map[string]bool)

	for _, file := range files {
		relPath, err := filepath.Rel(s.config.Local.MarkdownDir, file)
		if err != nil {
			continue
		}

		dir := filepath.Dir(relPath)
		if dir != "." {
			// Add all directory levels
			parts := strings.Split(dir, string(filepath.Separator))
			currentPath := ""
			for _, part := range parts {
				if currentPath == "" {
					currentPath = part
				} else {
					currentPath = filepath.Join(currentPath, part)
				}
				dirSet[currentPath] = true
			}
		}
	}

	// Convert to sorted slice
	var directories []string
	for dir := range dirSet {
		directories = append(directories, dir)
	}

	// Sort by depth (shallow first) so we create parent directories before children
	for i := 0; i < len(directories)-1; i++ {
		for j := i + 1; j < len(directories); j++ {
			if strings.Count(directories[i], string(filepath.Separator)) > strings.Count(directories[j], string(filepath.Separator)) {
				directories[i], directories[j] = directories[j], directories[i]
			}
		}
	}

	return directories
}

func (s *Syncer) createDirectoryPage(dirPath, parentDirPath string, directoryPages map[string]*confluence.Page, status SyncStatus, files []string) (*confluence.Page, error) {
	// Check if directory page already exists in our cache
	if page, exists := directoryPages[dirPath]; exists {
		return page, nil
	}

	// Skip creation if directory is up-to-date
	if status == StatusUpToDate {
		s.logger.Debug("Directory page is up-to-date, skipping: %s", dirPath)

		// Try to get existing page from Confluence using cached page ID
		if pageID := s.metadata.GetDirectoryPageID(dirPath); pageID != "" {
			if existingPage, err := s.confluence.GetPage(pageID); err == nil {
				directoryPages[dirPath] = existingPage
				return existingPage, nil
			}
		}

		// Fallback: try to find by title
		dirName := filepath.Base(dirPath)
		caser := cases.Title(language.English)
		title := caser.String(strings.ReplaceAll(dirName, "-", " "))

		if existingPage, err := s.confluence.FindPageByTitle(s.config.Confluence.SpaceKey, title); err == nil && existingPage != nil {
			directoryPages[dirPath] = existingPage
			return existingPage, nil
		}
	}

	// Create directory page title from the directory name
	dirName := filepath.Base(dirPath)
	caser := cases.Title(language.English)
	title := caser.String(strings.ReplaceAll(dirName, "-", " "))

	s.logger.Info("Creating directory page: %s (status: %s)", title, status)

	// Enhanced content for directory page with child items display
	content := fmt.Sprintf(`<h1>%s</h1>
<p>This section contains documentation for %s. The pages below are automatically listed and updated whenever child pages are added or modified.</p>

<h2>Contents</h2>
<ac:structured-macro ac:name="children" ac:schema-version="1">
<ac:parameter ac:name="all">true</ac:parameter>
<ac:parameter ac:name="sort">title</ac:parameter>
</ac:structured-macro>

<p><em>This page was automatically created by <a href="https://github.com/gdesouza/conflux">Conflux</a> to organize documentation hierarchy.</em></p>`, title, dirName)

	var page *confluence.Page
	var err error

	// Determine parent page ID
	var parentID string
	if parentDirPath != "" {
		if parentPage, exists := directoryPages[parentDirPath]; exists {
			parentID = parentPage.ID
		}
	}

	// Check if the directory page already exists in Confluence
	existingPage, err := s.confluence.FindPageByTitle(s.config.Confluence.SpaceKey, title)
	if err != nil {
		s.logger.Info("Could not check for existing directory page (creating new): %s", title)
	}

	if existingPage != nil {
		s.logger.Info("Directory page already exists, updating content: %s", title)
		// Update the existing directory page with new content
		var updatedPage *confluence.Page
		updatedPage, err = s.confluence.UpdatePage(existingPage.ID, title, content)
		if err != nil {
			s.logger.Info("Failed to update directory page, will recreate: %s", err)
		} else {
			s.logger.Info("Successfully updated directory page: %s (ID: %s)", title, updatedPage.ID)
			directoryPages[dirPath] = updatedPage
			// Update directory metadata
			s.metadata.UpdateDirectoryMetadata(dirPath, updatedPage.ID, title, files)
			return updatedPage, nil
		}
	}

	// Create the directory page
	if parentID != "" {
		page, err = s.confluence.CreatePageWithParent(s.config.Confluence.SpaceKey, title, content, parentID)
	} else {
		page, err = s.confluence.CreatePage(s.config.Confluence.SpaceKey, title, content)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create directory page: %w", err)
	}

	s.logger.Info("Successfully created directory page: %s (ID: %s)", title, page.ID)
	directoryPages[dirPath] = page

	// Update directory metadata
	s.metadata.UpdateDirectoryMetadata(dirPath, page.ID, title, files)

	return page, nil
}

func (s *Syncer) analyzeAllFiles(files []string) ([]PageSyncInfo, error) {
	var pages []PageSyncInfo

	// First, create directory page info with change detection
	directories := s.extractDirectories(files)
	for _, dir := range directories {
		dirName := filepath.Base(dir)
		caser := cases.Title(language.English)
		title := caser.String(strings.ReplaceAll(dirName, "-", " "))
		level := strings.Count(dir, string(filepath.Separator))

		// Check if directory needs stub page creation
		status := s.metadata.GetDirectoryStatus(dir, files)

		pages = append(pages, PageSyncInfo{
			Title:       title,
			FilePath:    dir,
			Status:      status,
			Level:       level,
			IsDirectory: true,
		})
	}

	// Then analyze markdown files with enhanced change detection
	for _, file := range files {
		pageInfo, err := s.analyzeFileWithMetadata(file)
		if err != nil {
			s.logger.Error("Failed to analyze file %s: %v", file, err)
			continue
		}
		pages = append(pages, pageInfo)
	}

	return pages, nil
}

func (s *Syncer) analyzeFileWithMetadata(filePath string) (PageSyncInfo, error) {
	s.logger.Debug("Analyzing file with metadata: %s", filePath)

	doc, err := markdown.ParseFile(filePath)
	if err != nil {
		return PageSyncInfo{}, fmt.Errorf("failed to parse markdown: %w", err)
	}

	// Use metadata-based change detection
	status, err := s.metadata.GetFileStatus(filePath)
	if err != nil {
		s.logger.Debug("Could not determine file status from metadata, assuming changed: %v", err)
		status = StatusChanged
	}

	// If metadata shows up-to-date, double-check with Confluence (if accessible)
	if status == StatusUpToDate {
		existingPage, err := s.confluence.FindPageByTitle(s.config.Confluence.SpaceKey, doc.Title)
		if err != nil {
			// Can't connect to Confluence, trust metadata
			s.logger.Debug("Cannot verify with Confluence, trusting metadata for: %s", doc.Title)
		} else if existingPage == nil {
			// Page doesn't exist in Confluence but metadata says up-to-date
			// This could happen if page was deleted externally
			status = StatusNew
		}
	}

	return PageSyncInfo{
		Title:    doc.Title,
		FilePath: filePath,
		Status:   status,
		Level:    0, // Will be calculated based on directory structure
	}, nil
}

func (s *Syncer) displayEnhancedHierarchy(pages []PageSyncInfo, newCount, changedCount, upToDateCount int, isDryRun bool) {
	if isDryRun {
		fmt.Printf("🔍 Dry Run - Space '%s':\n\n", s.config.Confluence.SpaceKey)
	} else {
		fmt.Printf("🏢 Space '%s' - Sync Preview:\n\n", s.config.Confluence.SpaceKey)
	}

	// Build and display tree
	tree := s.buildPageTree(pages)
	s.printEnhancedPageTree(tree, 0)

	// Display summary
	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("   🆕 New pages: %d\n", newCount)
	fmt.Printf("   📝 Changed pages: %d\n", changedCount)
	fmt.Printf("   ✅ Up-to-date pages: %d\n", upToDateCount)
	fmt.Printf("   📄 Total pages: %d\n\n", len(pages))

	if isDryRun {
		fmt.Println("💡 This is a dry run. No changes will be made.")
		fmt.Println("   Run without --dry-run to perform the actual sync.")
	}
}

func (s *Syncer) buildPageTree(pages []PageSyncInfo) []PageSyncInfo {
	// Create maps for organizing pages
	pageMap := make(map[string]*PageSyncInfo) // path -> page
	childrenMap := make(map[string][]string)  // parent path -> child paths

	// First, index all pages and determine parent-child relationships
	for i := range pages {
		page := &pages[i]

		if page.IsDirectory {
			// For directory pages, use the directory path as the key
			pageMap[page.FilePath] = page

			// Determine parent directory for nested directories
			if strings.Contains(page.FilePath, string(filepath.Separator)) {
				parentDir := filepath.Dir(page.FilePath)
				if parentDir != "." {
					page.ParentPath = parentDir
					childrenMap[parentDir] = append(childrenMap[parentDir], page.FilePath)
				}
			}
		} else {
			// For file pages, use the file path as the key
			pageMap[page.FilePath] = page

			// Determine parent directory for file pages
			relPath, err := filepath.Rel(s.config.Local.MarkdownDir, page.FilePath)
			if err != nil {
				relPath = page.FilePath
			}

			dir := filepath.Dir(relPath)
			if dir != "." {
				page.ParentPath = dir
				childrenMap[dir] = append(childrenMap[dir], page.FilePath)
			}
		}
	}

	// Build tree recursively
	var buildChildren func(string) []PageSyncInfo
	buildChildren = func(parentPath string) []PageSyncInfo {
		var children []PageSyncInfo
		if childPaths, exists := childrenMap[parentPath]; exists {
			for _, childPath := range childPaths {
				if childPage, exists := pageMap[childPath]; exists {
					// Create a copy of the child page
					child := *childPage
					// Recursively build children for this child
					child.Children = buildChildren(childPath)
					children = append(children, child)
				}
			}
		}
		return children
	}

	// Find root pages and build their children
	var rootPages []PageSyncInfo
	for _, page := range pageMap {
		if page.ParentPath == "" || pageMap[page.ParentPath] == nil {
			rootPage := *page
			rootPage.Children = buildChildren(page.FilePath)
			rootPages = append(rootPages, rootPage)
		}
	}

	return rootPages
}

func (s *Syncer) printEnhancedPageTree(pages []PageSyncInfo, level int) {
	for i, page := range pages {
		isLast := i == len(pages)-1

		// Build prefix with proper tree formatting
		prefix := ""
		for j := 0; j < level; j++ {
			prefix += "  "
		}
		if level > 0 {
			if isLast {
				prefix += "└── "
			} else {
				prefix += "├── "
			}
		}

		// Choose icon and status text based on status and type
		var icon, statusText string
		if page.IsDirectory {
			icon = "📁"
			statusText = " (directory page)"
		} else {
			switch page.Status {
			case StatusNew:
				icon = "🆕"
				statusText = " (new)"
			case StatusChanged:
				icon = "📝"
				statusText = " (modified)"
			case StatusUpToDate:
				icon = "✅"
				statusText = " (up-to-date)"
			default:
				icon = "📄"
				statusText = ""
			}
		}

		// Print the page with file path for non-directory pages
		if page.IsDirectory {
			fmt.Printf("%s%s %s%s\n", prefix, icon, page.Title, statusText)
		} else {
			relPath, err := filepath.Rel(s.config.Local.MarkdownDir, page.FilePath)
			if err != nil {
				// Fallback to filename if relative path calculation fails
				relPath = filepath.Base(page.FilePath)
			}
			fmt.Printf("%s%s %s%s\n", prefix, icon, page.Title, statusText)
			if level == 0 {
				fmt.Printf("%s    %s %s\n", prefix, icon, relPath)
			}
		}

		// Recursively print children
		if len(page.Children) > 0 {
			s.printEnhancedPageTree(page.Children, level+1)
		}
	}
}

func (s *Syncer) performEnhancedSync(pages []PageSyncInfo) error {
	s.logger.Info("Starting enhanced sync process...")

	var syncedCount, skippedCount, errorCount int

	// Build a map of page info for easy lookup
	pageInfoMap := make(map[string]PageSyncInfo)
	for _, page := range pages {
		pageInfoMap[page.FilePath] = page
	}

	// Extract all files for directory hash calculation
	var allFiles []string
	for _, page := range pages {
		if !page.IsDirectory {
			allFiles = append(allFiles, page.FilePath)
		}
	}

	// First create directory pages
	directoryPages := make(map[string]*confluence.Page)
	for _, page := range pages {
		if !page.IsDirectory {
			continue
		}

		// Extract directory path for directory pages
		dirPath := page.FilePath
		parentDir := filepath.Dir(dirPath)
		if parentDir == "." || parentDir == s.config.Local.MarkdownDir {
			parentDir = ""
		}

		// Skip if directory is up-to-date
		if page.Status == StatusUpToDate {
			s.logger.Debug("Skipping up-to-date directory: %s", dirPath)

			// Still need to load the page for parent-child relationships
			if pageID := s.metadata.GetDirectoryPageID(dirPath); pageID != "" {
				if existingPage, err := s.confluence.GetPage(pageID); err == nil {
					directoryPages[dirPath] = existingPage
				}
			}
			skippedCount++
			continue
		}

		confluencePage, err := s.createDirectoryPage(dirPath, parentDir, directoryPages, page.Status, allFiles)
		if err != nil {
			s.logger.Error("Failed to create directory page for %s: %v", dirPath, err)
			errorCount++
			continue
		}
		directoryPages[dirPath] = confluencePage
		syncedCount++
	}

	// Then sync markdown files
	for _, page := range pages {
		if page.IsDirectory {
			continue // Already handled above
		}

		if page.Status == StatusUpToDate {
			s.logger.Debug("Skipping up-to-date file: %s", page.Title)
			skippedCount++
			continue
		}

		err := s.syncFileWithMetadata(page.FilePath, directoryPages)
		if err != nil {
			s.logger.Error("Failed to sync file %s: %v", page.FilePath, err)
			errorCount++
			continue
		}
		syncedCount++
	}

	// Save metadata cache
	if err := s.metadata.Save(); err != nil {
		s.logger.Error("Failed to save sync metadata: %v", err)
	}

	// Display results
	fmt.Printf("\n✨ Sync completed!\n")
	fmt.Printf("   ✅ Synced: %d pages\n", syncedCount)
	fmt.Printf("   ⏭️  Skipped: %d pages (up-to-date)\n", skippedCount)
	if errorCount > 0 {
		fmt.Printf("   ❌ Errors: %d pages\n", errorCount)
	}

	return nil
}

func (s *Syncer) syncFileWithMetadata(filePath string, directoryPages map[string]*confluence.Page) error {
	s.logger.Info("Syncing file with metadata tracking: %s", filePath)

	doc, err := markdown.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse markdown: %w", err)
	}

	// Convert content with mermaid support
	pageID := s.metadata.GetPageID(filePath)
	var confluenceContent string
	if concreteClient, ok := s.confluence.(*confluence.Client); ok {
		confluenceContent = markdown.ConvertToConfluenceFormatWithMermaid(doc.Content, s.config, concreteClient, pageID)
	} else {
		confluenceContent = markdown.ConvertToConfluenceFormat(doc.Content)
	}

	// Determine parent page based on directory structure
	relPath, err := filepath.Rel(s.config.Local.MarkdownDir, filePath)
	if err != nil {
		relPath = filePath
	}

	dir := filepath.Dir(relPath)
	var parentID string
	if dir != "." {
		if parentPage, exists := directoryPages[dir]; exists {
			parentID = parentPage.ID
		}
	}

	var page *confluence.Page
	existingPage, err := s.confluence.FindPageByTitle(s.config.Confluence.SpaceKey, doc.Title)
	if err != nil {
		s.logger.Debug("Could not check for existing page (will create new): %s", doc.Title)
	}

	if existingPage != nil {
		s.logger.Info("Updating existing page: %s", doc.Title)
		page, err = s.confluence.UpdatePage(existingPage.ID, doc.Title, confluenceContent)

		// Handle archived/restricted page (403 error) by attempting replacement
		if confluence.IsPageUpdateForbidden(err) {
			s.logger.Info("Page appears to be archived or restricted, attempting to replace: %s", doc.Title)

			// Clear the cached page ID for this file to avoid future conflicts
			s.metadata.RemoveFileMetadata(filePath)

			page, err = s.handleArchivedPageReplacement(doc.Title, confluenceContent, parentID, existingPage.ID)
		}
	} else {
		s.logger.Info("Creating new page: %s", doc.Title)
		if parentID != "" {
			page, err = s.confluence.CreatePageWithParent(s.config.Confluence.SpaceKey, doc.Title, confluenceContent, parentID)
		} else {
			page, err = s.confluence.CreatePage(s.config.Confluence.SpaceKey, doc.Title, confluenceContent)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to sync page: %w", err)
	}

	// Post-process Mermaid diagrams for the created/updated page
	if s.config.Mermaid.Mode == "convert-to-image" && s.hasMermaidDiagrams(doc.Content) {
		s.logger.Info("Post-processing Mermaid diagrams for page: %s", doc.Title)
		err = s.postProcessMermaidDiagrams(page.ID, doc.Content)
		if err != nil {
			s.logger.Error("Failed to post-process Mermaid diagrams for %s: %v", doc.Title, err)
			// Don't fail the entire sync for Mermaid processing errors
		}
	}

	// Update metadata with successful sync
	if err := s.metadata.UpdateFileMetadata(filePath, page.ID, doc.Title); err != nil {
		s.logger.Error("Failed to update metadata for %s: %v", filePath, err)
	}

	s.logger.Info("Successfully synced: %s (ID: %s)", doc.Title, page.ID)
	return nil
}

func (s *Syncer) ClearCache() error {
	return s.metadata.ClearCache()
}

// hasMermaidDiagrams checks if the content contains any Mermaid diagram blocks
func (s *Syncer) hasMermaidDiagrams(content string) bool {
	return strings.Contains(content, "```mermaid")
}

// handleArchivedPageReplacement handles the case where an existing page is archived/restricted
// and we need to replace it with a new page with the same title
func (s *Syncer) handleArchivedPageReplacement(title, content, parentID, existingPageID string) (*confluence.Page, error) {
	// Attempt to create replacement page with original title
	s.logger.Info("Attempting to create replacement page: %s", title)
	var page *confluence.Page
	var err error

	if parentID != "" {
		page, err = s.confluence.CreatePageWithParent(s.config.Confluence.SpaceKey, title, content, parentID)
	} else {
		page, err = s.confluence.CreatePage(s.config.Confluence.SpaceKey, title, content)
	}

	// If we get a "title already exists" error, try with a timestamp suffix
	if err != nil && (strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "TITLE")) {
		s.logger.Info("Title conflict detected, creating page with timestamp suffix: %s", title)
		timestampSuffix := fmt.Sprintf(" (replaced %s)", time.Now().Format("2006-01-02 15:04"))
		newTitle := title + timestampSuffix

		if parentID != "" {
			page, err = s.confluence.CreatePageWithParent(s.config.Confluence.SpaceKey, newTitle, content, parentID)
		} else {
			page, err = s.confluence.CreatePage(s.config.Confluence.SpaceKey, newTitle, content)
		}

		if err == nil {
			s.logger.Info("Successfully created replacement page with new title: %s", newTitle)
		} else {
			s.logger.Error("Failed to create replacement page even with timestamp suffix: %v", err)
		}
	} else if err == nil {
		s.logger.Info("Successfully created replacement page: %s", title)
	} else {
		s.logger.Error("Failed to create replacement page: %v", err)
	}

	return page, err
}

// postProcessMermaidDiagrams re-processes a page to convert Mermaid diagrams to images
// This is called after page creation when we have a pageID for attachment uploads
func (s *Syncer) postProcessMermaidDiagrams(pageID, content string) error {
	s.logger.Debug("Post-processing Mermaid diagrams for page ID: %s", pageID)

	// Re-convert content with the actual pageID for Mermaid processing
	var updatedContent string
	if concreteClient, ok := s.confluence.(*confluence.Client); ok {
		updatedContent = markdown.ConvertToConfluenceFormatWithMermaid(content, s.config, concreteClient, pageID)
	} else {
		updatedContent = markdown.ConvertToConfluenceFormat(content)
	}

	// Get current page info to preserve title
	page, err := s.confluence.GetPage(pageID)
	if err != nil {
		return fmt.Errorf("failed to get page info for post-processing: %w", err)
	}

	// Update the page with Mermaid diagrams converted to images
	_, err = s.confluence.UpdatePage(pageID, page.Title, updatedContent)
	if err != nil {
		return fmt.Errorf("failed to update page with processed Mermaid diagrams: %w", err)
	}

	s.logger.Info("Successfully post-processed Mermaid diagrams for page: %s", page.Title)
	return nil
}
