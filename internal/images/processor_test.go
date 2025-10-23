package images

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conflux/internal/config"
)

func TestFindImageReferences(t *testing.T) {
	cfg := &config.ImageConfig{
		SupportedFormats: []string{"png", "jpg", "jpeg", "gif", "svg"},
		MaxFileSize:      10 * 1024 * 1024, // 10MB
	}
	processor := NewProcessor(cfg, nil)

	tests := []struct {
		name         string
		markdown     string
		markdownDir  string
		expectedRefs int
		expectedAlt  []string
		expectedPath []string
	}{
		{
			name:         "Single image reference",
			markdown:     "Here is an image: ![My Image](./images/test.png)",
			markdownDir:  "/home/user/docs",
			expectedRefs: 1,
			expectedAlt:  []string{"My Image"},
			expectedPath: []string{"./images/test.png"},
		},
		{
			name: "Multiple image references",
			markdown: `# Document
![First Image](image1.jpg)
Some text here.
![Second Image](./assets/image2.png)`,
			markdownDir:  "/home/user/docs",
			expectedRefs: 2,
			expectedAlt:  []string{"First Image", "Second Image"},
			expectedPath: []string{"image1.jpg", "./assets/image2.png"},
		},
		{
			name:         "No image references",
			markdown:     "This is just text with no images.",
			markdownDir:  "/home/user/docs",
			expectedRefs: 0,
		},
		{
			name:         "Image with empty alt text",
			markdown:     "![](empty-alt.gif)",
			markdownDir:  "/home/user/docs",
			expectedRefs: 1,
			expectedAlt:  []string{""},
			expectedPath: []string{"empty-alt.gif"},
		},
		{
			name:         "Absolute path image",
			markdown:     "![Absolute](/full/path/to/image.svg)",
			markdownDir:  "/home/user/docs",
			expectedRefs: 1,
			expectedAlt:  []string{"Absolute"},
			expectedPath: []string{"/full/path/to/image.svg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs, err := processor.FindImageReferences(tt.markdown, tt.markdownDir)
			if err != nil {
				t.Fatalf("FindImageReferences() error = %v", err)
			}

			if len(refs) != tt.expectedRefs {
				t.Errorf("FindImageReferences() found %d references, expected %d", len(refs), tt.expectedRefs)
			}

			for i, ref := range refs {
				if i < len(tt.expectedAlt) && ref.AltText != tt.expectedAlt[i] {
					t.Errorf("Reference %d alt text = %q, expected %q", i, ref.AltText, tt.expectedAlt[i])
				}
				if i < len(tt.expectedPath) && ref.FilePath != tt.expectedPath[i] {
					t.Errorf("Reference %d file path = %q, expected %q", i, ref.FilePath, tt.expectedPath[i])
				}
			}
		})
	}
}

func TestValidateImageFile(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "conflux_image_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test image files
	validImagePath := filepath.Join(tempDir, "valid.png")
	if err := os.WriteFile(validImagePath, []byte("fake png content"), 0600); err != nil {
		t.Fatalf("Failed to create test image: %v", err)
	}

	largeImagePath := filepath.Join(tempDir, "large.jpg")
	if err := os.WriteFile(largeImagePath, make([]byte, 2*1024*1024), 0600); err != nil { // 2MB
		t.Fatalf("Failed to create large test image: %v", err)
	}

	unsupportedImagePath := filepath.Join(tempDir, "test.bmp")
	if err := os.WriteFile(unsupportedImagePath, []byte("fake bmp content"), 0600); err != nil {
		t.Fatalf("Failed to create unsupported test image: %v", err)
	}

	cfg := &config.ImageConfig{
		SupportedFormats: []string{"png", "jpg", "jpeg", "gif", "svg"},
		MaxFileSize:      1024 * 1024, // 1MB limit
	}
	processor := NewProcessor(cfg, nil)

	tests := []struct {
		name          string
		ref           *ImageReference
		expectError   bool
		errorContains string
	}{
		{
			name: "Valid image file",
			ref: &ImageReference{
				AbsolutePath: validImagePath,
			},
			expectError: false,
		},
		{
			name: "Non-existent file",
			ref: &ImageReference{
				AbsolutePath: filepath.Join(tempDir, "nonexistent.png"),
			},
			expectError:   true,
			errorContains: "not found",
		},
		{
			name: "Unsupported format",
			ref: &ImageReference{
				AbsolutePath: unsupportedImagePath,
			},
			expectError:   true,
			errorContains: "not supported",
		},
		{
			name: "File too large",
			ref: &ImageReference{
				AbsolutePath: largeImagePath,
			},
			expectError:   true,
			errorContains: "exceeds maximum size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processor.ValidateImageFile(tt.ref)

			if tt.expectError {
				if err == nil {
					t.Errorf("ValidateImageFile() expected error but got none")
				} else if tt.errorContains != "" && !containsString(err.Error(), tt.errorContains) {
					t.Errorf("ValidateImageFile() error = %q, expected to contain %q", err.Error(), tt.errorContains)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateImageFile() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestGetImageFilename(t *testing.T) {
	tests := []struct {
		imagePath string
		expected  string
	}{
		{"/path/to/image.png", "image.png"},
		{"./relative/path/photo.jpg", "photo.jpg"},
		{"simple.gif", "simple.gif"},
		{"/home/user/docs/images/diagram.svg", "diagram.svg"},
	}

	for _, tt := range tests {
		t.Run(tt.imagePath, func(t *testing.T) {
			result := GetImageFilename(tt.imagePath)
			if result != tt.expected {
				t.Errorf("GetImageFilename(%q) = %q, expected %q", tt.imagePath, result, tt.expected)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
