package content

import (
	"fmt"
	"path/filepath"
	"strings"
)

const (
	attachmentsSuffix = ".attachments"
	metadataFilename  = "metadata.json"
)

type ArtifactPaths struct {
	MarkdownDir    string
	MarkdownPath   string
	AttachmentsDir string
	MetadataPath   string
}

func PathsFor(markdownPath string) (ArtifactPaths, error) {
	if strings.TrimSpace(markdownPath) == "" {
		return ArtifactPaths{}, fmt.Errorf("markdown path is required")
	}
	if !strings.EqualFold(filepath.Ext(markdownPath), ".md") {
		return ArtifactPaths{}, fmt.Errorf("markdown path must have a .md extension: %s", markdownPath)
	}

	cleanPath := filepath.Clean(markdownPath)
	dir := filepath.Dir(cleanPath)
	base := strings.TrimSuffix(filepath.Base(cleanPath), filepath.Ext(cleanPath))
	if base == "" || base == "." {
		return ArtifactPaths{}, fmt.Errorf("markdown path must include a filename: %s", markdownPath)
	}

	attachmentsDir := filepath.Join(dir, base+attachmentsSuffix)
	return ArtifactPaths{
		MarkdownDir:    dir,
		MarkdownPath:   cleanPath,
		AttachmentsDir: attachmentsDir,
		MetadataPath:   filepath.Join(attachmentsDir, metadataFilename),
	}, nil
}
