package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/pkg/logger"
)

// Mock implementations for testing

// ConfluenceClient interface for testing
type ConfluenceClient interface {
	CreatePage(spaceKey, title, content string) (*confluence.Page, error)
	CreatePageWithParent(spaceKey, title, content, parentID string) (*confluence.Page, error)
	UpdatePage(pageID, title, content string) (*confluence.Page, error)
	FindPageByTitle(spaceKey, title string) (*confluence.Page, error)
	GetPage(pageID string) (*confluence.Page, error)
	UploadAttachment(pageID, filePath, contentType string) (*confluence.Attachment, error)
}

type mockConfluenceClient struct {
	pages            map[string]*confluence.Page // title -> page
	archivedPages    map[string]bool             // pageID -> archived status
	forbiddenUpdates map[string]bool             // pageID -> forbidden to update
	createPageError  error
	updatePageError  error
	findPageError    error
}

func newMockConfluenceClient() *mockConfluenceClient {
	return &mockConfluenceClient{
		pages:            make(map[string]*confluence.Page),
		archivedPages:    make(map[string]bool),
		forbiddenUpdates: make(map[string]bool),
	}
}

func (m *mockConfluenceClient) CreatePage(spaceKey, title, content string) (*confluence.Page, error) {
	if m.createPageError != nil {
		return nil, m.createPageError
	}

	// Check for title conflicts
	if _, exists := m.pages[title]; exists {
		return nil, fmt.Errorf("page with title '%s' already exists with the same TITLE", title)
	}

	pageID := fmt.Sprintf("page-%d", len(m.pages)+1)
	page := &confluence.Page{
		ID:    pageID,
		Title: title,
	}
	page.Space.Key = spaceKey
	page.Body.Storage.Value = content
	m.pages[title] = page
	return page, nil
}

func (m *mockConfluenceClient) CreatePageWithParent(spaceKey, title, content, parentID string) (*confluence.Page, error) {
	// For simplicity, just call CreatePage - parent logic is tested separately
	return m.CreatePage(spaceKey, title, content)
}

func (m *mockConfluenceClient) UpdatePage(pageID, title, content string) (*confluence.Page, error) {
	if m.updatePageError != nil {
		return nil, m.updatePageError
	}

	// Check if page update is forbidden (archived page scenario)
	if m.forbiddenUpdates[pageID] {
		return nil, &confluence.PageUpdateForbiddenError{
			PageID: pageID,
			Msg:    "Page is archived or access restricted",
		}
	}

	// Find the page by ID
	for existingTitle, page := range m.pages {
		if page.ID == pageID {
			// Update the page
			page.Title = title
			page.Body.Storage.Value = content

			// If title changed, update the map
			if existingTitle != title {
				delete(m.pages, existingTitle)
				m.pages[title] = page
			}

			return page, nil
		}
	}

	return nil, fmt.Errorf("page with ID %s not found", pageID)
}

func (m *mockConfluenceClient) FindPageByTitle(spaceKey, title string) (*confluence.Page, error) {
	if m.findPageError != nil {
		return nil, m.findPageError
	}

	if page, exists := m.pages[title]; exists {
		return page, nil
	}
	return nil, nil
}

func (m *mockConfluenceClient) GetPage(pageID string) (*confluence.Page, error) {
	for _, page := range m.pages {
		if page.ID == pageID {
			return page, nil
		}
	}
	return nil, fmt.Errorf("page with ID %s not found", pageID)
}

func (m *mockConfluenceClient) UploadAttachment(pageID, filePath string) (*confluence.Attachment, error) {
	return &confluence.Attachment{
		ID:    fmt.Sprintf("attachment-%s", pageID),
		Title: filepath.Base(filePath),
	}, nil
}

func (m *mockConfluenceClient) AddArchivedPage(title string) {
	if page, exists := m.pages[title]; exists {
		m.archivedPages[page.ID] = true
		m.forbiddenUpdates[page.ID] = true
	}
}

// Test setup helper - returns both syncer and mock, but doesn't assign mock to syncer
func setupSyncerTest(t *testing.T) (*Syncer, *mockConfluenceClient, string) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		Confluence: config.ConfluenceConfig{
			BaseURL:  "https://test.atlassian.net",
			Username: "test@example.com",
			APIToken: "test-token",
			SpaceKey: "TEST",
		},
		Local: config.LocalConfig{
			MarkdownDir: tempDir,
			Exclude:     []string{},
		},
		Mermaid: config.MermaidConfig{
			Mode: "preserve",
		},
	}

	logger := logger.New(false) // quiet
	mockClient := newMockConfluenceClient()
	syncer := NewWithClient(cfg, logger, mockClient)

	return syncer, mockClient, tempDir
}

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Confluence: config.ConfluenceConfig{
			BaseURL:  "https://test.atlassian.net",
			Username: "test@example.com",
			APIToken: "test-token",
			SpaceKey: "TEST",
		},
		Local: config.LocalConfig{
			MarkdownDir: "/path/to/docs",
		},
		Mermaid: config.MermaidConfig{
			Mode: "preserve",
		},
	}

	logger := logger.New(false)
	syncer := New(cfg, logger)

	if syncer.config != cfg {
		t.Error("Expected config to be set")
	}

	if syncer.logger != logger {
		t.Error("Expected logger to be set")
	}

	if syncer.confluence == nil {
		t.Error("Expected confluence client to be created")
	}

	if syncer.metadata == nil {
		t.Error("Expected metadata to be created")
	}

	if syncer.metadata.SpaceKey != "TEST" {
		t.Errorf("Expected metadata space key 'TEST', got %q", syncer.metadata.SpaceKey)
	}
}

func TestSyncer_AnalyzeFileWithMetadata_NewFile(t *testing.T) {
	syncer, mockClient, tempDir := setupSyncerTest(t)

	// Create test file
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test Page\n\nThis is test content."
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Configure mock to return no existing page
	mockClient.findPageError = nil

	pageInfo, err := syncer.analyzeFileWithMetadata(testFile)
	if err != nil {
		t.Fatalf("Failed to analyze file: %v", err)
	}

	if pageInfo.Title != "Test Page" {
		t.Errorf("Expected title 'Test Page', got %q", pageInfo.Title)
	}

	if pageInfo.FilePath != testFile {
		t.Errorf("Expected file path %q, got %q", testFile, pageInfo.FilePath)
	}

	if pageInfo.Status != StatusNew {
		t.Errorf("Expected status StatusNew, got %v", pageInfo.Status)
	}
}

func TestSyncer_AnalyzeFileWithMetadata_ExistingFile(t *testing.T) {
	syncer, mockClient, tempDir := setupSyncerTest(t)

	// Create test file
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test Page\n\nThis is test content."
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add file to metadata as up-to-date
	err = syncer.metadata.UpdateFileMetadata(testFile, "page-1", "Test Page")
	if err != nil {
		t.Fatalf("Failed to update metadata: %v", err)
	}

	// Add existing page to mock client
	mockClient.pages["Test Page"] = &confluence.Page{
		ID:    "page-1",
		Title: "Test Page",
	}

	pageInfo, err := syncer.analyzeFileWithMetadata(testFile)
	if err != nil {
		t.Fatalf("Failed to analyze file: %v", err)
	}

	if pageInfo.Status != StatusUpToDate {
		t.Errorf("Expected status StatusUpToDate, got %v", pageInfo.Status)
	}
}

func TestSyncer_AnalyzeFileWithMetadata_ChangedFile(t *testing.T) {
	syncer, mockClient, tempDir := setupSyncerTest(t)

	// Create test file
	testFile := filepath.Join(tempDir, "test.md")
	originalContent := "# Test Page\n\nOriginal content."
	err := os.WriteFile(testFile, []byte(originalContent), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add file to metadata
	err = syncer.metadata.UpdateFileMetadata(testFile, "page-1", "Test Page")
	if err != nil {
		t.Fatalf("Failed to update metadata: %v", err)
	}

	// Modify the file
	newContent := "# Test Page\n\nModified content."
	err = os.WriteFile(testFile, []byte(newContent), 0600)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Add existing page to mock client
	mockClient.pages["Test Page"] = &confluence.Page{
		ID:    "page-1",
		Title: "Test Page",
	}

	pageInfo, err := syncer.analyzeFileWithMetadata(testFile)
	if err != nil {
		t.Fatalf("Failed to analyze file: %v", err)
	}

	if pageInfo.Status != StatusChanged {
		t.Errorf("Expected status StatusChanged, got %v", pageInfo.Status)
	}
}

func TestSyncer_ExtractDirectories(t *testing.T) {
	syncer, _, tempDir := setupSyncerTest(t)

	files := []string{
		filepath.Join(tempDir, "file1.md"),
		filepath.Join(tempDir, "docs", "file2.md"),
		filepath.Join(tempDir, "docs", "api", "file3.md"),
		filepath.Join(tempDir, "guides", "file4.md"),
	}

	directories := syncer.extractDirectories(files)

	expectedDirs := []string{"docs", "guides", "docs/api"}
	if len(directories) != len(expectedDirs) {
		t.Errorf("Expected %d directories, got %d: %v", len(expectedDirs), len(directories), directories)
	}

	// Check that all expected directories are present
	dirSet := make(map[string]bool)
	for _, dir := range directories {
		dirSet[dir] = true
	}

	for _, expectedDir := range expectedDirs {
		if !dirSet[expectedDir] {
			t.Errorf("Expected directory %q not found in result", expectedDir)
		}
	}
}

func TestSyncer_BuildPageTree(t *testing.T) {
	syncer, _, tempDir := setupSyncerTest(t)

	pages := []PageSyncInfo{
		{
			Title:       "Root File",
			FilePath:    filepath.Join(tempDir, "root.md"),
			Status:      StatusNew,
			IsDirectory: false,
		},
		{
			Title:       "Docs",
			FilePath:    "docs",
			Status:      StatusNew,
			IsDirectory: true,
		},
		{
			Title:       "Doc File",
			FilePath:    filepath.Join(tempDir, "docs", "doc.md"),
			Status:      StatusNew,
			IsDirectory: false,
			ParentPath:  "docs",
		},
	}

	tree := syncer.buildPageTree(pages)

	if len(tree) != 2 { // Root File and Docs directory
		t.Errorf("Expected 2 root pages, got %d", len(tree))
	}

	// Find the Docs directory in the tree
	var docsPage *PageSyncInfo
	for i := range tree {
		if tree[i].IsDirectory && tree[i].Title == "Docs" {
			docsPage = &tree[i]
			break
		}
	}

	if docsPage == nil {
		t.Fatal("Expected to find Docs directory page in tree")
	}

	if len(docsPage.Children) != 1 {
		t.Errorf("Expected Docs directory to have 1 child, got %d", len(docsPage.Children))
	}

	if docsPage.Children[0].Title != "Doc File" {
		t.Errorf("Expected child to be 'Doc File', got %q", docsPage.Children[0].Title)
	}
}

func TestSyncer_SyncFileWithMetadata_NewPage(t *testing.T) {
	syncer, mockClient, tempDir := setupSyncerTest(t)

	// Create test file
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test Page\n\nThis is test content."
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	directoryPages := make(map[string]*confluence.Page)

	err = syncer.syncFileWithMetadata(testFile, directoryPages)
	if err != nil {
		t.Fatalf("Failed to sync file: %v", err)
	}

	// Verify page was created
	if len(mockClient.pages) != 1 {
		t.Errorf("Expected 1 page to be created, got %d", len(mockClient.pages))
	}

	page, exists := mockClient.pages["Test Page"]
	if !exists {
		t.Fatal("Expected page 'Test Page' to be created")
	}

	if page.Title != "Test Page" {
		t.Errorf("Expected page title 'Test Page', got %q", page.Title)
	}

	// Verify metadata was updated
	pageID := syncer.metadata.GetPageID(testFile)
	if pageID != page.ID {
		t.Errorf("Expected metadata to contain page ID %q, got %q", page.ID, pageID)
	}
}

func TestSyncer_SyncFileWithMetadata_UpdatePage(t *testing.T) {
	syncer, mockClient, tempDir := setupSyncerTest(t)

	// Create test file
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test Page\n\nUpdated content."
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add existing page to mock client
	existingPage := &confluence.Page{
		ID:    "page-1",
		Title: "Test Page",
	}
	existingPage.Body.Storage.Value = "old content"
	mockClient.pages["Test Page"] = existingPage

	// Add metadata for existing page
	err = syncer.metadata.UpdateFileMetadata(testFile, "page-1", "Test Page")
	if err != nil {
		t.Fatalf("Failed to update metadata: %v", err)
	}

	directoryPages := make(map[string]*confluence.Page)

	err = syncer.syncFileWithMetadata(testFile, directoryPages)
	if err != nil {
		t.Fatalf("Failed to sync file: %v", err)
	}

	// Verify page was updated (should contain new content)
	updatedPage := mockClient.pages["Test Page"]
	if !strings.Contains(updatedPage.Body.Storage.Value, "Updated content") {
		t.Error("Expected page content to be updated")
	}
}

func TestSyncer_SyncFileWithMetadata_ArchivedPageRecovery(t *testing.T) {
	syncer, mockClient, tempDir := setupSyncerTest(t)

	// Create test file
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test Page\n\nContent for archived page."
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add existing archived page to mock client
	existingPage := &confluence.Page{
		ID:    "archived-page-1",
		Title: "Test Page",
	}
	mockClient.pages["Test Page"] = existingPage
	mockClient.AddArchivedPage("Test Page") // This will make updates forbidden

	directoryPages := make(map[string]*confluence.Page)

	err = syncer.syncFileWithMetadata(testFile, directoryPages)
	if err != nil {
		t.Fatalf("Failed to sync file with archived page recovery: %v", err)
	}

	// Verify that a new page was created (the old one should still exist too)
	if len(mockClient.pages) != 2 { // Original archived + new page
		t.Errorf("Expected 2 pages after recovery, got %d", len(mockClient.pages))
	}

	// Check that the new page exists (should be titled "Test Page" or "Test Page (v2)")
	var newPageFound bool
	for title, page := range mockClient.pages {
		if page.ID != "archived-page-1" && (title == "Test Page" || strings.HasPrefix(title, "Test Page (v")) {
			newPageFound = true
			if !strings.Contains(page.Body.Storage.Value, "Content for archived page") {
				t.Error("Expected new page to contain updated content")
			}
			break
		}
	}

	if !newPageFound {
		t.Error("Expected new page to be created for archived page recovery")
	}

	// Verify metadata no longer contains reference to archived page
	cachedPageID := syncer.metadata.GetPageID(testFile)
	if cachedPageID == "archived-page-1" {
		t.Error("Expected metadata to be updated with new page ID after archived page recovery")
	}
}

func TestSyncer_SyncFileWithMetadata_TitleConflictRetry(t *testing.T) {
	syncer, mockClient, tempDir := setupSyncerTest(t)

	// Create test file
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Existing Title\n\nContent."
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Add existing archived page to mock client
	archivedPage := &confluence.Page{
		ID:    "archived-page-1",
		Title: "Existing Title",
	}
	mockClient.pages["Existing Title"] = archivedPage
	mockClient.AddArchivedPage("Existing Title")

	// Add a page with the first retry title to force multiple retries
	conflictPage := &confluence.Page{
		ID:    "conflict-page-1",
		Title: "Existing Title (v2)",
	}
	mockClient.pages["Existing Title (v2)"] = conflictPage

	directoryPages := make(map[string]*confluence.Page)

	err = syncer.syncFileWithMetadata(testFile, directoryPages)
	if err != nil {
		t.Fatalf("Failed to sync file with title conflict: %v", err)
	}

	// Should have created page with title "Existing Title (v3)"
	newPage, exists := mockClient.pages["Existing Title (v3)"]
	if !exists {
		// Check what pages were actually created
		var titles []string
		for title := range mockClient.pages {
			titles = append(titles, title)
		}
		t.Fatalf("Expected page 'Existing Title (v3)' to be created. Available pages: %v", titles)
	}

	if newPage.ID == "archived-page-1" || newPage.ID == "conflict-page-1" {
		t.Error("Expected new page to have different ID from existing pages")
	}
}

func TestSyncer_CreateDirectoryPage(t *testing.T) {
	syncer, mockClient, _ := setupSyncerTest(t)

	directoryPages := make(map[string]*confluence.Page)

	page, err := syncer.createDirectoryPage("docs", "", directoryPages)
	if err != nil {
		t.Fatalf("Failed to create directory page: %v", err)
	}

	if page.Title != "Docs" {
		t.Errorf("Expected title 'Docs', got %q", page.Title)
	}

	// Verify page was added to mock client
	createdPage, exists := mockClient.pages["Docs"]
	if !exists {
		t.Fatal("Expected directory page to be created in mock client")
	}

	if !strings.Contains(createdPage.Body.Storage.Value, "children") {
		t.Error("Expected directory page to contain children macro")
	}

	// Verify page was cached
	if directoryPages["docs"] != page {
		t.Error("Expected directory page to be cached")
	}
}

func TestSyncer_CreateDirectoryPage_ExistingPage(t *testing.T) {
	syncer, mockClient, _ := setupSyncerTest(t)

	// Add existing directory page
	existingPage := &confluence.Page{
		ID:    "existing-dir-1",
		Title: "Docs",
	}
	existingPage.Body.Storage.Value = "old directory content"
	mockClient.pages["Docs"] = existingPage

	directoryPages := make(map[string]*confluence.Page)

	page, err := syncer.createDirectoryPage("docs", "", directoryPages)
	if err != nil {
		t.Fatalf("Failed to update existing directory page: %v", err)
	}

	if page.ID != "existing-dir-1" {
		t.Errorf("Expected to get existing page ID, got %q", page.ID)
	}

	// Verify content was updated
	updatedPage := mockClient.pages["Docs"]
	if !strings.Contains(updatedPage.Body.Storage.Value, "children") {
		t.Error("Expected updated directory page to contain children macro")
	}

	if strings.Contains(updatedPage.Body.Storage.Value, "old directory content") {
		t.Error("Expected old content to be replaced")
	}
}

func TestSyncer_ClearCache(t *testing.T) {
	syncer, _, tempDir := setupSyncerTest(t)

	// Add some metadata
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test"
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err = syncer.metadata.UpdateFileMetadata(testFile, "page-1", "Test")
	if err != nil {
		t.Fatalf("Failed to update metadata: %v", err)
	}

	err = syncer.metadata.Save()
	if err != nil {
		t.Fatalf("Failed to save metadata: %v", err)
	}

	// Verify metadata exists
	if len(syncer.metadata.Files) != 1 {
		t.Fatal("Expected metadata to contain file")
	}

	// Clear cache
	err = syncer.ClearCache()
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Verify cache is empty
	if len(syncer.metadata.Files) != 0 {
		t.Errorf("Expected cache to be empty after clear, got %d files", len(syncer.metadata.Files))
	}

	// Verify cache file was updated
	newSyncer := &Syncer{
		metadata: NewSyncMetadata(tempDir, "TEST"),
	}
	err = newSyncer.metadata.Load()
	if err != nil {
		t.Fatalf("Failed to load metadata after clear: %v", err)
	}

	if len(newSyncer.metadata.Files) != 0 {
		t.Errorf("Expected saved cache to be empty, got %d files", len(newSyncer.metadata.Files))
	}
}

func TestSyncer_HasMermaidDiagrams(t *testing.T) {
	syncer, _, _ := setupSyncerTest(t)

	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "no mermaid",
			content:  "# Title\n\nSome content",
			expected: false,
		},
		{
			name:     "has mermaid",
			content:  "# Title\n\nSome content\n\n```mermaid\ngraph TD\n    A --> B\n```\n\nMore content",
			expected: true,
		},
		{
			name:     "mermaid in code span",
			content:  "Use `mermaid` for diagrams",
			expected: false,
		},
		{
			name:     "multiple mermaid blocks",
			content:  "# Title\n\n```mermaid\ngraph TD\n    A --> B\n```\n\nAnd another:\n\n```mermaid\nsequenceDiagram\n    A->>B: Hello\n```",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncer.hasMermaidDiagrams(tt.content)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestSyncer_Integration_FullSync(t *testing.T) {
	syncer, mockClient, tempDir := setupSyncerTest(t)

	// Create a more complex directory structure
	docsDir := filepath.Join(tempDir, "docs")
	apiDir := filepath.Join(tempDir, "docs", "api")

	err := os.MkdirAll(apiDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create directories: %v", err)
	}

	// Create test files
	files := map[string]string{
		filepath.Join(tempDir, "README.md"):          "# Project\n\nMain readme",
		filepath.Join(docsDir, "getting-started.md"): "# Getting Started\n\nHow to start",
		filepath.Join(apiDir, "reference.md"):        "# API Reference\n\nAPI docs",
	}

	for filePath, content := range files {
		err = os.WriteFile(filePath, []byte(content), 0600)
		if err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}

	// Perform sync (dry run first)
	err = syncer.Sync(true, true) // dryRun=true, force=true
	if err != nil {
		t.Fatalf("Failed to perform dry run sync: %v", err)
	}

	// No pages should be created in dry run
	if len(mockClient.pages) != 0 {
		t.Errorf("Expected no pages to be created in dry run, got %d", len(mockClient.pages))
	}

	// Now perform actual sync
	err = syncer.Sync(false, true) // dryRun=false, force=true
	if err != nil {
		t.Fatalf("Failed to perform actual sync: %v", err)
	}

	// Verify pages were created
	// Should have: 3 markdown files + 2 directory pages (docs, api)
	expectedPages := 5
	if len(mockClient.pages) != expectedPages {
		var pageNames []string
		for name := range mockClient.pages {
			pageNames = append(pageNames, name)
		}
		t.Errorf("Expected %d pages to be created, got %d: %v", expectedPages, len(mockClient.pages), pageNames)
	}

	// Verify specific pages exist
	expectedTitles := []string{"Project", "Getting Started", "API Reference", "Docs", "Api"}
	for _, title := range expectedTitles {
		if _, exists := mockClient.pages[title]; !exists {
			t.Errorf("Expected page '%s' to be created", title)
		}
	}

	// Verify metadata was updated
	if len(syncer.metadata.Files) == 0 {
		t.Error("Expected metadata to contain file entries after sync")
	}

	// Verify that a second sync recognizes files as up-to-date
	err = syncer.Sync(true, true) // Another dry run
	if err != nil {
		t.Fatalf("Failed to perform second dry run sync: %v", err)
	}
}
