package sync

import (
	"fmt"
	"sort"
	"strings"
)

// RenameDetection represents a detected page/folder rename situation
type RenameDetection struct {
	Type           string // "page" or "directory" 
	LocalPath      string // Current local path/title
	ExpectedTitle  string // What Conflux expects based on local structure
	ActualTitle    string // What exists in Confluence
	PageID         string // Confluence page ID
	CachedTitle    string // Title stored in cache
	Severity       string // "warning" or "critical"
	Recommendation string // What user should do
}

// DetectRenames analyzes the sync plan and detects potential rename issues
func (s *Syncer) DetectRenames(pages []PageSyncInfo) ([]RenameDetection, error) {
	var detections []RenameDetection
	
	s.logger.Debug("Running rename detection analysis...")
	
	// Check directory renames
	for _, page := range pages {
		if page.IsDirectory {
			detection := s.checkDirectoryRename(page)
			if detection != nil {
				detections = append(detections, *detection)
			}
		} else {
			detection := s.checkPageRename(page)
			if detection != nil {
				detections = append(detections, *detection)
			}
		}
	}
	
	// Check for orphaned pages (cached pages that don't match current structure)
	orphanDetections := s.checkOrphanedPages(pages)
	detections = append(detections, orphanDetections...)
	
	s.logger.Debug("Rename detection found %d potential issues", len(detections))
	return detections, nil
}

// checkDirectoryRename detects when a directory has been renamed locally
func (s *Syncer) checkDirectoryRename(page PageSyncInfo) *RenameDetection {
	cachedPageID := s.metadata.GetDirectoryPageID(page.FilePath)
	if cachedPageID == "" {
		return nil // No cached page, not a rename
	}
	
	// Get the actual page from Confluence
	existingPage, err := s.confluence.GetPage(cachedPageID)
	if err != nil {
		s.logger.Debug("Could not retrieve page for rename check: %s", cachedPageID)
		return nil
	}
	
	// Compare expected title vs actual title
	if existingPage.Title != page.Title {
		// Get cached title if available
		cachedTitle := ""
		if metadata, exists := s.metadata.Directories[page.FilePath]; exists {
			cachedTitle = metadata.Title
		}
		
		severity := "warning"
		recommendation := fmt.Sprintf("Directory renamed from '%s' to '%s'. Children macro may not work properly.", 
			existingPage.Title, page.Title)
			
		// If there are many children, this becomes critical
		if s.hasChildPages(cachedPageID) {
			severity = "critical"
			recommendation += " Consider manually moving child pages or clearing cache."
		}
		
		return &RenameDetection{
			Type:           "directory",
			LocalPath:      page.FilePath,
			ExpectedTitle:  page.Title,
			ActualTitle:    existingPage.Title,
			PageID:         cachedPageID,
			CachedTitle:    cachedTitle,
			Severity:       severity,
			Recommendation: recommendation,
		}
	}
	
	return nil
}

// checkPageRename detects when a page has been renamed locally or in Confluence
func (s *Syncer) checkPageRename(page PageSyncInfo) *RenameDetection {
	cachedPageID := s.metadata.GetPageID(page.FilePath)
	if cachedPageID == "" {
		return nil // No cached page, not a rename
	}
	
	// Get the actual page from Confluence
	existingPage, err := s.confluence.GetPage(cachedPageID)
	if err != nil {
		s.logger.Debug("Could not retrieve page for rename check: %s", cachedPageID)
		return nil
	}
	
	// Compare expected title vs actual title
	if existingPage.Title != page.Title {
		// Get cached title if available
		cachedTitle := ""
		if metadata, exists := s.metadata.Files[page.FilePath]; exists {
			cachedTitle = metadata.Title
		}
		
		// Determine if rename was local or remote
		var recommendation string
		if cachedTitle == page.Title {
			// Local title matches cache, so rename happened in Confluence
			recommendation = fmt.Sprintf("Page renamed in Confluence from '%s' to '%s'. Conflux will preserve the Confluence title.", 
				page.Title, existingPage.Title)
		} else if cachedTitle == existingPage.Title {
			// Confluence title matches cache, so rename happened locally  
			recommendation = fmt.Sprintf("Page renamed locally from '%s' to '%s'. Conflux will update the Confluence page title.", 
				existingPage.Title, page.Title)
		} else {
			// Both sides changed - conflict situation
			recommendation = fmt.Sprintf("Title conflict detected. Local: '%s', Confluence: '%s', Cached: '%s'. Manual resolution may be needed.", 
				page.Title, existingPage.Title, cachedTitle)
		}
		
		return &RenameDetection{
			Type:           "page",
			LocalPath:      page.FilePath,
			ExpectedTitle:  page.Title,
			ActualTitle:    existingPage.Title,
			PageID:         cachedPageID,
			CachedTitle:    cachedTitle,
			Severity:       "warning",
			Recommendation: recommendation,
		}
	}
	
	return nil
}

// checkOrphanedPages finds pages in cache that no longer exist locally
func (s *Syncer) checkOrphanedPages(pages []PageSyncInfo) []RenameDetection {
	var detections []RenameDetection
	
	// Build set of current local paths
	localPaths := make(map[string]bool)
	for _, page := range pages {
		localPaths[page.FilePath] = true
	}
	
	// Check cached files that are no longer local
	for filePath, metadata := range s.metadata.Files {
		if !localPaths[filePath] {
			// This file was cached but no longer exists locally
			detections = append(detections, RenameDetection{
				Type:           "page",
				LocalPath:      filePath + " (DELETED)",
				ExpectedTitle:  "",
				ActualTitle:    metadata.Title,
				PageID:         metadata.PageID,
				CachedTitle:    metadata.Title,
				Severity:       "warning",
				Recommendation: fmt.Sprintf("Local file deleted but Confluence page '%s' still exists. Consider manual cleanup.", metadata.Title),
			})
		}
	}
	
	// Check cached directories that are no longer local
	for dirPath, metadata := range s.metadata.Directories {
		if !localPaths[dirPath] {
			// This directory was cached but no longer exists locally
			detections = append(detections, RenameDetection{
				Type:           "directory",
				LocalPath:      dirPath + " (DELETED)",
				ExpectedTitle:  "",
				ActualTitle:    metadata.Title,
				PageID:         metadata.PageID,
				CachedTitle:    metadata.Title,
				Severity:       "critical",
				Recommendation: fmt.Sprintf("Local directory deleted but Confluence directory page '%s' still exists with potential child pages. Manual cleanup recommended.", metadata.Title),
			})
		}
	}
	
	return detections
}

// hasChildPages checks if a page has child pages (for determining severity)
func (s *Syncer) hasChildPages(pageID string) bool {
	children, err := s.confluence.GetChildPages(pageID)
	return err == nil && len(children) > 0
}

// DisplayRenameDetections shows rename detection results to user
func (s *Syncer) DisplayRenameDetections(detections []RenameDetection) {
	if len(detections) == 0 {
		return
	}
	
	// Sort by severity (critical first, then warnings)
	sort.Slice(detections, func(i, j int) bool {
		if detections[i].Severity != detections[j].Severity {
			return detections[i].Severity == "critical"
		}
		return detections[i].LocalPath < detections[j].LocalPath
	})
	
	fmt.Println()
	fmt.Println("🔍 Rename Detection Analysis:")
	fmt.Println(strings.Repeat("=", 50))
	
	criticalCount := 0
	warningCount := 0
	
	for i, detection := range detections {
		if detection.Severity == "critical" {
			criticalCount++
			fmt.Printf("🚨 CRITICAL #%d: %s Rename Detected\n", criticalCount, strings.Title(detection.Type))
		} else {
			warningCount++
			fmt.Printf("⚠️  WARNING #%d: %s Rename Detected\n", warningCount, strings.Title(detection.Type))
		}
		
		fmt.Printf("   📁 Local Path: %s\n", detection.LocalPath)
		if detection.ExpectedTitle != "" {
			fmt.Printf("   📝 Expected Title: %s\n", detection.ExpectedTitle)
		}
		fmt.Printf("   🌐 Confluence Title: %s\n", detection.ActualTitle)
		if detection.CachedTitle != "" && detection.CachedTitle != detection.ActualTitle {
			fmt.Printf("   💾 Cached Title: %s\n", detection.CachedTitle)
		}
		fmt.Printf("   🔗 Page ID: %s\n", detection.PageID)
		fmt.Printf("   💡 %s\n", detection.Recommendation)
		
		if i < len(detections)-1 {
			fmt.Println()
		}
	}
	
	fmt.Println()
	fmt.Printf("📊 Summary: %d critical issues, %d warnings detected\n", criticalCount, warningCount)
	
	if criticalCount > 0 {
		fmt.Println()
		fmt.Println("🎯 Recommended Actions:")
		fmt.Println("1. Review critical issues above - they may cause broken parent-child relationships")
		fmt.Println("2. Use 'conflux inspect' command to examine current page hierarchy")
		fmt.Println("3. Consider manually fixing page relationships in Confluence")
		fmt.Println("4. Or clear cache with 'rm -rf .conflux/' and re-sync")
	}
	
	fmt.Println()
}