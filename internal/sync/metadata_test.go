package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSyncMetadata(t *testing.T) {
	markdownDir := "/path/to/markdown"
	spaceKey := "TEST"

	sm := NewSyncMetadata(markdownDir, spaceKey)

	if sm.SpaceKey != spaceKey {
		t.Errorf("Expected SpaceKey %q, got %q", spaceKey, sm.SpaceKey)
	}

	if sm.Version != "1.0" {
		t.Errorf("Expected Version '1.0', got %q", sm.Version)
	}

	if sm.Files == nil {
		t.Error("Expected Files map to be initialized")
	}

	expectedCacheDir := filepath.Join(markdownDir, ".conflux")
	if sm.cacheDir != expectedCacheDir {
		t.Errorf("Expected cacheDir %q, got %q", expectedCacheDir, sm.cacheDir)
	}

	expectedCacheFile := filepath.Join(expectedCacheDir, "sync-cache.json")
	if sm.cacheFile != expectedCacheFile {
		t.Errorf("Expected cacheFile %q, got %q", expectedCacheFile, sm.cacheFile)
	}

	if sm.markdownDir != markdownDir {
		t.Errorf("Expected markdownDir %q, got %q", markdownDir, sm.markdownDir)
	}
}

func TestSyncMetadata_Load_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSyncMetadata(tempDir, "TEST")

	// Should not error when file doesn't exist
	err := sm.Load()
	if err != nil {
		t.Errorf("Expected no error for non-existent file, got: %v", err)
	}
}

func TestSyncMetadata_Load_InvalidJSON(t *testing.T) {
	tempDir := t.TempDir()
	cacheDir := filepath.Join(tempDir, ".conflux")
	cacheFile := filepath.Join(cacheDir, "sync-cache.json")

	// Create directory and invalid JSON file
	err := os.MkdirAll(cacheDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	invalidJSON := `{"invalid": json}`
	err = os.WriteFile(cacheFile, []byte(invalidJSON), 0600)
	if err != nil {
		t.Fatalf("Failed to write invalid JSON: %v", err)
	}

	sm := NewSyncMetadata(tempDir, "TEST")
	err = sm.Load()
	if err == nil {
		t.Error("Expected error for invalid JSON, got none")
	}
}

func TestSyncMetadata_SaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSyncMetadata(tempDir, "TEST")

	// Add some test data
	testTime := time.Now().Truncate(time.Second) // Truncate for JSON marshaling
	sm.Files["test.md"] = FileMetadata{
		Hash:     "testhash123",
		LastSync: testTime,
		PageID:   "12345",
		Title:    "Test Page",
		ModTime:  testTime,
		Size:     1024,
	}
	sm.LastSync = testTime

	// Save
	err := sm.Save()
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Load into new instance
	sm2 := NewSyncMetadata(tempDir, "TEST")
	err = sm2.Load()
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	// Verify data
	if sm2.SpaceKey != "TEST" {
		t.Errorf("Expected SpaceKey 'TEST', got %q", sm2.SpaceKey)
	}

	metadata, exists := sm2.Files["test.md"]
	if !exists {
		t.Fatal("Expected file metadata to exist")
	}

	if metadata.Hash != "testhash123" {
		t.Errorf("Expected Hash 'testhash123', got %q", metadata.Hash)
	}

	if metadata.PageID != "12345" {
		t.Errorf("Expected PageID '12345', got %q", metadata.PageID)
	}

	if metadata.Title != "Test Page" {
		t.Errorf("Expected Title 'Test Page', got %q", metadata.Title)
	}
}

func TestSyncMetadata_Save_CreateDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Use a subdirectory that doesn't exist yet
	markdownDir := filepath.Join(tempDir, "docs")
	sm := NewSyncMetadata(markdownDir, "TEST")

	err := sm.Save()
	if err != nil {
		t.Fatalf("Failed to save with non-existent directory: %v", err)
	}

	// Verify directory was created
	cacheDir := filepath.Join(markdownDir, ".conflux")
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		t.Error("Expected cache directory to be created")
	}

	// Verify file was created
	cacheFile := filepath.Join(cacheDir, "sync-cache.json")
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		t.Error("Expected cache file to be created")
	}
}

func TestSyncMetadata_NormalizeFilePath(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSyncMetadata(tempDir, "TEST")

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "relative path unchanged",
			input:    "docs/test.md",
			expected: "docs/test.md",
		},
		{
			name:     "absolute path under markdown dir",
			input:    filepath.Join(tempDir, "docs", "test.md"),
			expected: filepath.Join("docs", "test.md"),
		},
		{
			name:     "absolute path outside markdown dir",
			input:    "/other/path/test.md",
			expected: "../../../other/path/test.md", // filepath.Rel returns relative path even when outside
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sm.normalizeFilePath(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSyncMetadata_NormalizeFilePath_EmptyMarkdownDir(t *testing.T) {
	sm := NewSyncMetadata("", "TEST")

	input := "/some/path/test.md"
	result := sm.normalizeFilePath(input)

	if result != input {
		t.Errorf("Expected path unchanged when markdownDir is empty, got %q", result)
	}
}

func TestSyncMetadata_CalculateFileHash(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.md")

	// Create test file
	content := "# Test File\n\nThis is test content."
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	sm := NewSyncMetadata(tempDir, "TEST")
	hash, err := sm.CalculateFileHash(testFile)
	if err != nil {
		t.Fatalf("Failed to calculate hash: %v", err)
	}

	if hash == "" {
		t.Error("Expected non-empty hash")
	}

	if len(hash) != 64 { // SHA256 produces 64-character hex string
		t.Errorf("Expected hash length 64, got %d", len(hash))
	}

	// Verify hash is consistent
	hash2, err := sm.CalculateFileHash(testFile)
	if err != nil {
		t.Fatalf("Failed to calculate hash second time: %v", err)
	}

	if hash != hash2 {
		t.Errorf("Hash changed between calculations: %q vs %q", hash, hash2)
	}
}

func TestSyncMetadata_CalculateFileHash_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSyncMetadata(tempDir, "TEST")

	_, err := sm.CalculateFileHash(filepath.Join(tempDir, "nonexistent.md"))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestSyncMetadata_GetFileStatus(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test File"

	// Create test file
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	sm := NewSyncMetadata(tempDir, "TEST")

	// New file should return StatusNew
	status, err := sm.GetFileStatus(testFile)
	if err != nil {
		t.Fatalf("Failed to get file status: %v", err)
	}
	if status != StatusNew {
		t.Errorf("Expected StatusNew, got %v", status)
	}

	// Add file to metadata
	err = sm.UpdateFileMetadata(testFile, "12345", "Test Page")
	if err != nil {
		t.Fatalf("Failed to update file metadata: %v", err)
	}

	// Should now return StatusUpToDate
	status, err = sm.GetFileStatus(testFile)
	if err != nil {
		t.Fatalf("Failed to get file status: %v", err)
	}
	if status != StatusUpToDate {
		t.Errorf("Expected StatusUpToDate, got %v", status)
	}

	// Modify file
	newContent := "# Test File\n\nNew content added"
	err = os.WriteFile(testFile, []byte(newContent), 0600)
	if err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Should now return StatusChanged
	status, err = sm.GetFileStatus(testFile)
	if err != nil {
		t.Fatalf("Failed to get file status: %v", err)
	}
	if status != StatusChanged {
		t.Errorf("Expected StatusChanged, got %v", status)
	}
}

func TestSyncMetadata_GetFileStatus_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSyncMetadata(tempDir, "TEST")

	_, err := sm.GetFileStatus(filepath.Join(tempDir, "nonexistent.md"))
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestSyncMetadata_UpdateFileMetadata(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test File"

	// Create test file
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	sm := NewSyncMetadata(tempDir, "TEST")

	pageID := "12345"
	title := "Test Page"

	err = sm.UpdateFileMetadata(testFile, pageID, title)
	if err != nil {
		t.Fatalf("Failed to update file metadata: %v", err)
	}

	// Verify metadata was stored correctly
	normalizedPath := sm.normalizeFilePath(testFile)
	metadata, exists := sm.Files[normalizedPath]
	if !exists {
		t.Fatal("Expected file metadata to exist")
	}

	if metadata.PageID != pageID {
		t.Errorf("Expected PageID %q, got %q", pageID, metadata.PageID)
	}

	if metadata.Title != title {
		t.Errorf("Expected Title %q, got %q", title, metadata.Title)
	}

	if metadata.Hash == "" {
		t.Error("Expected non-empty hash")
	}

	if metadata.Size != int64(len(content)) {
		t.Errorf("Expected Size %d, got %d", len(content), metadata.Size)
	}
}

func TestSyncMetadata_UpdateFileMetadata_NonExistentFile(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSyncMetadata(tempDir, "TEST")

	err := sm.UpdateFileMetadata(filepath.Join(tempDir, "nonexistent.md"), "12345", "Title")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}
}

func TestSyncMetadata_RemoveFileMetadata(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test File"

	// Create test file
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	sm := NewSyncMetadata(tempDir, "TEST")

	// Add metadata
	err = sm.UpdateFileMetadata(testFile, "12345", "Test Page")
	if err != nil {
		t.Fatalf("Failed to update file metadata: %v", err)
	}

	// Verify it exists
	normalizedPath := sm.normalizeFilePath(testFile)
	if _, exists := sm.Files[normalizedPath]; !exists {
		t.Fatal("Expected file metadata to exist before removal")
	}

	// Remove metadata
	sm.RemoveFileMetadata(testFile)

	// Verify it's gone
	if _, exists := sm.Files[normalizedPath]; exists {
		t.Error("Expected file metadata to be removed")
	}
}

func TestSyncMetadata_GetPageID(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.md")
	content := "# Test File"

	// Create test file
	err := os.WriteFile(testFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	sm := NewSyncMetadata(tempDir, "TEST")

	// Should return empty string for non-existent metadata
	pageID := sm.GetPageID(testFile)
	if pageID != "" {
		t.Errorf("Expected empty PageID, got %q", pageID)
	}

	// Add metadata
	expectedPageID := "12345"
	err = sm.UpdateFileMetadata(testFile, expectedPageID, "Test Page")
	if err != nil {
		t.Fatalf("Failed to update file metadata: %v", err)
	}

	// Should now return the PageID
	pageID = sm.GetPageID(testFile)
	if pageID != expectedPageID {
		t.Errorf("Expected PageID %q, got %q", expectedPageID, pageID)
	}
}

func TestSyncMetadata_GetCachedFiles(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSyncMetadata(tempDir, "TEST")

	// Should return empty slice initially
	files := sm.GetCachedFiles()
	if len(files) != 0 {
		t.Errorf("Expected 0 cached files, got %d", len(files))
	}

	// Add some metadata
	sm.Files["file1.md"] = FileMetadata{PageID: "1", Title: "File 1"}
	sm.Files["file2.md"] = FileMetadata{PageID: "2", Title: "File 2"}
	sm.Files["dir/file3.md"] = FileMetadata{PageID: "3", Title: "File 3"}

	files = sm.GetCachedFiles()
	if len(files) != 3 {
		t.Errorf("Expected 3 cached files, got %d", len(files))
	}

	// Verify all files are present (order doesn't matter)
	fileSet := make(map[string]bool)
	for _, file := range files {
		fileSet[file] = true
	}

	expectedFiles := []string{"file1.md", "file2.md", "dir/file3.md"}
	for _, expected := range expectedFiles {
		if !fileSet[expected] {
			t.Errorf("Expected file %q in cached files list", expected)
		}
	}
}

func TestSyncMetadata_ClearCache(t *testing.T) {
	tempDir := t.TempDir()
	sm := NewSyncMetadata(tempDir, "TEST")

	// Add some test data
	sm.Files["test.md"] = FileMetadata{PageID: "12345", Title: "Test Page"}

	// Clear cache
	err := sm.ClearCache()
	if err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Verify files map is empty
	if len(sm.Files) != 0 {
		t.Errorf("Expected 0 files after clear, got %d", len(sm.Files))
	}

	// Verify cache file was updated
	data, err := os.ReadFile(sm.cacheFile)
	if err != nil {
		t.Fatalf("Failed to read cache file: %v", err)
	}

	var savedMetadata SyncMetadata
	err = json.Unmarshal(data, &savedMetadata)
	if err != nil {
		t.Fatalf("Failed to parse saved cache: %v", err)
	}

	if len(savedMetadata.Files) != 0 {
		t.Errorf("Expected 0 files in saved cache, got %d", len(savedMetadata.Files))
	}
}

func TestSyncMetadata_Integration(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tempDir, "file1.md")
	file2 := filepath.Join(tempDir, "docs", "file2.md")

	err := os.MkdirAll(filepath.Dir(file2), 0755)
	if err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	err = os.WriteFile(file1, []byte("# File 1"), 0600)
	if err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	err = os.WriteFile(file2, []byte("# File 2"), 0600)
	if err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	sm := NewSyncMetadata(tempDir, "SPACE")

	// Both files should be new
	status1, err := sm.GetFileStatus(file1)
	if err != nil {
		t.Fatalf("Failed to get file1 status: %v", err)
	}
	if status1 != StatusNew {
		t.Errorf("Expected file1 to be new, got %v", status1)
	}

	status2, err := sm.GetFileStatus(file2)
	if err != nil {
		t.Fatalf("Failed to get file2 status: %v", err)
	}
	if status2 != StatusNew {
		t.Errorf("Expected file2 to be new, got %v", status2)
	}

	// Update metadata for both files
	err = sm.UpdateFileMetadata(file1, "page1", "File 1")
	if err != nil {
		t.Fatalf("Failed to update file1 metadata: %v", err)
	}

	err = sm.UpdateFileMetadata(file2, "page2", "File 2")
	if err != nil {
		t.Fatalf("Failed to update file2 metadata: %v", err)
	}

	// Save and reload
	err = sm.Save()
	if err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	sm2 := NewSyncMetadata(tempDir, "SPACE")
	err = sm2.Load()
	if err != nil {
		t.Fatalf("Failed to load: %v", err)
	}

	// Both files should now be up to date
	status1, err = sm2.GetFileStatus(file1)
	if err != nil {
		t.Fatalf("Failed to get file1 status after reload: %v", err)
	}
	if status1 != StatusUpToDate {
		t.Errorf("Expected file1 to be up-to-date after reload, got %v", status1)
	}

	status2, err = sm2.GetFileStatus(file2)
	if err != nil {
		t.Fatalf("Failed to get file2 status after reload: %v", err)
	}
	if status2 != StatusUpToDate {
		t.Errorf("Expected file2 to be up-to-date after reload, got %v", status2)
	}

	// Verify page IDs are correct
	if sm2.GetPageID(file1) != "page1" {
		t.Errorf("Expected file1 PageID 'page1', got %q", sm2.GetPageID(file1))
	}

	if sm2.GetPageID(file2) != "page2" {
		t.Errorf("Expected file2 PageID 'page2', got %q", sm2.GetPageID(file2))
	}

	// Verify cached files list
	cachedFiles := sm2.GetCachedFiles()
	if len(cachedFiles) != 2 {
		t.Errorf("Expected 2 cached files, got %d", len(cachedFiles))
	}
}
