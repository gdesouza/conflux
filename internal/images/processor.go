package images

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"conflux/internal/config"
	"conflux/pkg/logger"
)

// ImageReference represents an image found in markdown content
type ImageReference struct {
	MarkdownSyntax string // Original markdown: ![alt text](image.png)
	AltText        string // Alt text from the markdown
	FilePath       string // Path to the image file (relative or absolute)
	AbsolutePath   string // Resolved absolute path to the image file
}

// Processor handles image processing and validation
type Processor struct {
	config *config.ImageConfig
	logger *logger.Logger
}

// NewProcessor creates a new image processor
func NewProcessor(cfg *config.ImageConfig, log *logger.Logger) *Processor {
	return &Processor{
		config: cfg,
		logger: log,
	}
}

// FindImageReferences searches markdown content for image references ![alt](path)
func (p *Processor) FindImageReferences(markdown string, markdownDir string) ([]*ImageReference, error) {
	// Regex to match ![alt text](image.png) or ![alt text](./images/file.png)
	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	matches := imageRegex.FindAllStringSubmatch(markdown, -1)

	var references []*ImageReference

	for _, match := range matches {
		if len(match) != 3 {
			continue
		}

		ref := &ImageReference{
			MarkdownSyntax: match[0], // Full match: ![alt](path)
			AltText:        match[1], // Alt text
			FilePath:       match[2], // File path
		}

		// Resolve absolute path
		var err error
		if filepath.IsAbs(ref.FilePath) {
			ref.AbsolutePath = ref.FilePath
		} else {
			// Relative path - resolve relative to the markdown file's directory
			ref.AbsolutePath, err = filepath.Abs(filepath.Join(markdownDir, ref.FilePath))
			if err != nil {
				if p.logger != nil {
					p.logger.Debug("Failed to resolve absolute path for image '%s': %v", ref.FilePath, err)
				}
				continue
			}
		}

		references = append(references, ref)
	}

	return references, nil
}

// ValidateImageFile checks if an image file exists and is supported
func (p *Processor) ValidateImageFile(ref *ImageReference) error {
	// Check if file exists
	info, err := os.Stat(ref.AbsolutePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("image file not found: %s", ref.AbsolutePath)
		}
		return fmt.Errorf("failed to access image file %s: %w", ref.AbsolutePath, err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("image path is not a regular file: %s", ref.AbsolutePath)
	}

	// Check file size
	if p.config.MaxFileSize > 0 && info.Size() > p.config.MaxFileSize {
		return fmt.Errorf("image file %s exceeds maximum size limit (%d bytes): %d bytes",
			ref.AbsolutePath, p.config.MaxFileSize, info.Size())
	}

	// Check file extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(ref.AbsolutePath), "."))
	if !p.isFormatSupported(ext) {
		return fmt.Errorf("image format '%s' is not supported for file %s. Supported formats: %v",
			ext, ref.AbsolutePath, p.config.SupportedFormats)
	}

	if p.logger != nil {
		p.logger.Debug("Validated image file '%s' (%d bytes, format: %s)", ref.AbsolutePath, info.Size(), ext)
	}

	return nil
}

// ValidateImageReferences validates all image references found in markdown
func (p *Processor) ValidateImageReferences(references []*ImageReference) ([]*ImageReference, error) {
	var validRefs []*ImageReference
	var errors []string

	for _, ref := range references {
		if err := p.ValidateImageFile(ref); err != nil {
			errors = append(errors, err.Error())
			if p.logger != nil {
				p.logger.Debug("Skipping invalid image reference: %s", err.Error())
			}
			continue
		}
		validRefs = append(validRefs, ref)
	}

	if len(errors) > 0 {
		if p.logger != nil {
			p.logger.Debug("Found %d invalid image references out of %d total", len(errors), len(references))
		}
		// For now, just log errors but continue with valid references
		// In the future, we might want to make this configurable (fail vs warn)
	}

	return validRefs, nil
}

// isFormatSupported checks if the given file extension is in the supported formats list
func (p *Processor) isFormatSupported(ext string) bool {
	for _, format := range p.config.SupportedFormats {
		if strings.ToLower(format) == ext {
			return true
		}
	}
	return false
}

// GetImageFilename returns the filename portion of an image path for use in Confluence attachments
func GetImageFilename(imagePath string) string {
	return filepath.Base(imagePath)
}

// CalculateImageHash calculates the SHA256 hash of an image file
func CalculateImageHash(imagePath string) (string, error) {
	file, err := os.Open(imagePath)
	if err != nil {
		return "", fmt.Errorf("failed to open image file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
