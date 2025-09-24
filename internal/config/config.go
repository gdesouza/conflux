package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Confluence ConfluenceConfig `yaml:"confluence"`
	Local      LocalConfig      `yaml:"local"`
	Mermaid    MermaidConfig    `yaml:"mermaid"`
	Images     ImageConfig      `yaml:"images"`
}

type ConfluenceConfig struct {
	BaseURL  string `yaml:"base_url"`
	Username string `yaml:"username"`
	APIToken string `yaml:"api_token"`
	SpaceKey string `yaml:"space_key"`
}

type LocalConfig struct {
	MarkdownDir string   `yaml:"markdown_dir"`
	Exclude     []string `yaml:"exclude"`
}

type MermaidConfig struct {
	Mode    string  `yaml:"mode"`     // "preserve" or "convert-to-image"
	Format  string  `yaml:"format"`   // "svg", "png", "pdf"
	CLIPath string  `yaml:"cli_path"` // path to mermaid CLI executable
	Theme   string  `yaml:"theme"`    // mermaid theme
	Width   int     `yaml:"width"`    // image width in pixels
	Height  int     `yaml:"height"`   // image height in pixels
	Scale   float64 `yaml:"scale"`    // puppeteer scale factor for higher resolution
}

type ImageConfig struct {
	SupportedFormats []string `yaml:"supported_formats"` // File extensions: png, jpg, jpeg, gif, svg, webp
	MaxFileSize      int64    `yaml:"max_file_size"`     // Maximum file size in bytes (default: 10MB)
	ResizeLarge      bool     `yaml:"resize_large"`      // Whether to resize large images
	MaxWidth         int      `yaml:"max_width"`         // Max width for resizing (default: 1200px)
	MaxHeight        int      `yaml:"max_height"`        // Max height for resizing (default: 800px)
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set mermaid defaults
	config.setMermaidDefaults()

	// Set image defaults
	config.setImageDefaults()

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

func (c *Config) setMermaidDefaults() {
	if c.Mermaid.Mode == "" {
		c.Mermaid.Mode = "convert-to-image"
	}
	if c.Mermaid.Format == "" {
		c.Mermaid.Format = "svg"
	}
	if c.Mermaid.CLIPath == "" {
		c.Mermaid.CLIPath = "mmdc"
	}
	if c.Mermaid.Theme == "" {
		c.Mermaid.Theme = "default"
	}
	if c.Mermaid.Width == 0 {
		c.Mermaid.Width = 1200 // Increased from default 800 for larger diagrams
	}
	if c.Mermaid.Height == 0 {
		c.Mermaid.Height = 800 // Increased from default 600 for larger diagrams
	}
	if c.Mermaid.Scale == 0 {
		c.Mermaid.Scale = 2.0 // 2x scale for higher resolution (default is 1)
	}
}

func (c *Config) setImageDefaults() {
	if len(c.Images.SupportedFormats) == 0 {
		c.Images.SupportedFormats = []string{"png", "jpg", "jpeg", "gif", "svg", "webp"}
	}
	if c.Images.MaxFileSize == 0 {
		c.Images.MaxFileSize = 10 * 1024 * 1024 // 10MB default
	}
	if c.Images.MaxWidth == 0 {
		c.Images.MaxWidth = 1200
	}
	if c.Images.MaxHeight == 0 {
		c.Images.MaxHeight = 800
	}
}

func (c *Config) validateMermaid() error {
	validModes := map[string]bool{
		"preserve":         true,
		"convert-to-image": true,
	}
	if !validModes[c.Mermaid.Mode] {
		return fmt.Errorf("mermaid.mode must be 'preserve' or 'convert-to-image'")
	}

	validFormats := map[string]bool{
		"svg": true,
		"png": true,
		"pdf": true,
	}
	if !validFormats[c.Mermaid.Format] {
		return fmt.Errorf("mermaid.format must be 'svg', 'png', or 'pdf'")
	}

	return nil
}

func (c *Config) validateImages() error {
	validFormats := map[string]bool{
		"png":  true,
		"jpg":  true,
		"jpeg": true,
		"gif":  true,
		"svg":  true,
		"webp": true,
	}

	for _, format := range c.Images.SupportedFormats {
		if !validFormats[format] {
			return fmt.Errorf("images.supported_formats contains invalid format '%s', must be one of: png, jpg, jpeg, gif, svg, webp", format)
		}
	}

	if c.Images.MaxFileSize < 0 {
		return fmt.Errorf("images.max_file_size cannot be negative")
	}

	if c.Images.MaxWidth < 0 {
		return fmt.Errorf("images.max_width cannot be negative")
	}

	if c.Images.MaxHeight < 0 {
		return fmt.Errorf("images.max_height cannot be negative")
	}

	return nil
}

// LoadForSync loads config with relaxed validation for space_key (can be overridden by CLI)
func LoadForSync(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set mermaid defaults
	config.setMermaidDefaults()

	// Set image defaults
	config.setImageDefaults()

	if err := config.validateForSync(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// LoadForListPages loads config with relaxed validation (space_key not required)
func LoadForListPages(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set mermaid defaults
	config.setMermaidDefaults()

	// Set image defaults
	config.setImageDefaults()

	if err := config.validateForListPages(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

func (c *Config) validate() error {
	if c.Confluence.BaseURL == "" {
		return fmt.Errorf("confluence.base_url is required")
	}
	if c.Confluence.Username == "" {
		return fmt.Errorf("confluence.username is required")
	}
	if c.Confluence.APIToken == "" {
		return fmt.Errorf("confluence.api_token is required")
	}
	if c.Confluence.SpaceKey == "" {
		return fmt.Errorf("confluence.space_key is required")
	}

	if err := c.validateMermaid(); err != nil {
		return err
	}

	if err := c.validateImages(); err != nil {
		return err
	}

	return nil
}

// validateForSync validates config for sync command (space_key not required if provided via CLI)
func (c *Config) validateForSync() error {
	if c.Confluence.BaseURL == "" {
		return fmt.Errorf("confluence.base_url is required")
	}
	if c.Confluence.Username == "" {
		return fmt.Errorf("confluence.username is required")
	}
	if c.Confluence.APIToken == "" {
		return fmt.Errorf("confluence.api_token is required")
	}
	// Note: space_key is NOT required for sync command (can be provided via CLI)

	if err := c.validateMermaid(); err != nil {
		return err
	}

	if err := c.validateImages(); err != nil {
		return err
	}

	return nil
}

// validateForListPages validates config for list-pages command (space_key not required)
func (c *Config) validateForListPages() error {
	if c.Confluence.BaseURL == "" {
		return fmt.Errorf("confluence.base_url is required")
	}
	if c.Confluence.Username == "" {
		return fmt.Errorf("confluence.username is required")
	}
	if c.Confluence.APIToken == "" {
		return fmt.Errorf("confluence.api_token is required")
	}
	// Note: space_key is NOT required for list-pages command

	if err := c.validateMermaid(); err != nil {
		return err
	}

	if err := c.validateImages(); err != nil {
		return err
	}

	return nil
}
