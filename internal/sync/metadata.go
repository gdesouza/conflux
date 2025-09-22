package sync

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type FileMetadata struct {
	Hash     string    `json:"hash"`
	LastSync time.Time `json:"last_sync"`
	PageID   string    `json:"page_id,omitempty"`
	Title    string    `json:"title"`
	ModTime  time.Time `json:"mod_time"`
	Size     int64     `json:"size"`
}

type SyncMetadata struct {
	Files       map[string]FileMetadata `json:"files"`
	LastSync    time.Time               `json:"last_sync"`
	SpaceKey    string                  `json:"space_key"`
	Version     string                  `json:"version"`
	cacheDir    string
	cacheFile   string
	markdownDir string
}

func NewSyncMetadata(markdownDir, spaceKey string) *SyncMetadata {
	cacheDir := filepath.Join(markdownDir, ".conflux")
	cacheFile := filepath.Join(cacheDir, "sync-cache.json")

	return &SyncMetadata{
		Files:       make(map[string]FileMetadata),
		SpaceKey:    spaceKey,
		Version:     "1.0",
		cacheDir:    cacheDir,
		cacheFile:   cacheFile,
		markdownDir: markdownDir,
	}
}

func (sm *SyncMetadata) Load() error {
	data, err := os.ReadFile(sm.cacheFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read sync cache: %w", err)
	}

	if err := json.Unmarshal(data, sm); err != nil {
		return fmt.Errorf("failed to parse sync cache: %w", err)
	}

	return nil
}

func (sm *SyncMetadata) Save() error {
	if err := os.MkdirAll(sm.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	sm.LastSync = time.Now()

	data, err := json.MarshalIndent(sm, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sync cache: %w", err)
	}

	if err := os.WriteFile(sm.cacheFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write sync cache: %w", err)
	}

	return nil
}

// normalizeFilePath converts absolute file paths to relative paths based on markdownDir
func (sm *SyncMetadata) normalizeFilePath(filePath string) string {
	if sm.markdownDir == "" {
		return filePath
	}

	// If already relative or not under markdownDir, return as-is
	if !filepath.IsAbs(filePath) {
		return filePath
	}

	relPath, err := filepath.Rel(sm.markdownDir, filePath)
	if err != nil {
		// If we can't make it relative, use the absolute path
		return filePath
	}

	return relPath
}

func (sm *SyncMetadata) CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func (sm *SyncMetadata) GetFileStatus(filePath string) (SyncStatus, error) {
	// Normalize the file path for consistent cache lookup
	normalizedPath := sm.normalizeFilePath(filePath)

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	// Calculate current hash
	currentHash, err := sm.CalculateFileHash(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Check if we have cached metadata for this file (using normalized path)
	metadata, exists := sm.Files[normalizedPath]
	if !exists {
		return StatusNew, nil
	}

	// Compare hashes to detect changes
	if metadata.Hash != currentHash {
		return StatusChanged, nil
	}

	// Also check modification time as a backup
	if fileInfo.ModTime().After(metadata.ModTime) {
		return StatusChanged, nil
	}

	return StatusUpToDate, nil
}

func (sm *SyncMetadata) UpdateFileMetadata(filePath, pageID, title string) error {
	// Normalize the file path for consistent cache storage
	normalizedPath := sm.normalizeFilePath(filePath)

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	hash, err := sm.CalculateFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	sm.Files[normalizedPath] = FileMetadata{
		Hash:     hash,
		LastSync: time.Now(),
		PageID:   pageID,
		Title:    title,
		ModTime:  fileInfo.ModTime(),
		Size:     fileInfo.Size(),
	}

	return nil
}

func (sm *SyncMetadata) RemoveFileMetadata(filePath string) {
	normalizedPath := sm.normalizeFilePath(filePath)
	delete(sm.Files, normalizedPath)
}

func (sm *SyncMetadata) GetPageID(filePath string) string {
	normalizedPath := sm.normalizeFilePath(filePath)
	if metadata, exists := sm.Files[normalizedPath]; exists {
		return metadata.PageID
	}
	return ""
}

func (sm *SyncMetadata) GetCachedFiles() []string {
	var files []string
	for filePath := range sm.Files {
		files = append(files, filePath)
	}
	return files
}

func (sm *SyncMetadata) ClearCache() error {
	sm.Files = make(map[string]FileMetadata)
	return sm.Save()
}
