package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Confluence ConfluenceConfig `yaml:"confluence"`
	Local      LocalConfig      `yaml:"local"`    // Backward compatibility: single local section
	Projects   []ProjectConfig  `yaml:"projects"` // New multi-project support
	Mermaid    MermaidConfig    `yaml:"mermaid"`
	Images     ImageConfig      `yaml:"images"`
}

type ConfluenceConfig struct {
	BaseURL  string `yaml:"base_url"`
	Username string `yaml:"username"`
	APIToken string `yaml:"api_token"`
	SpaceKey string `yaml:"space_key"` // Optional when using projects
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

// ProjectConfig defines an individual documentation project mapping local markdown to a Confluence space.
// Example YAML:
// projects:
//   - name: "docs"
//     space_key: "DOCS"
//     local:
//       markdown_dir: "./docs"
//       exclude: ["README.md"]
//
// Rules:
// - Project name must be unique and non-empty
// - space_key required per project
// - local.markdown_dir required per project
// - If projects list is non-empty, top-level confluence.space_key becomes optional
// - First project acts as default if none specified at runtime
// - Top-level Local + SpaceKey kept for backward compatibility (single project scenario)
// - Images/Mermaid settings remain global

type ProjectConfig struct {
	Name     string      `yaml:"name"`
	SpaceKey string      `yaml:"space_key"`
	Local    LocalConfig `yaml:"local"`
}

// ResolveConfigPath returns the path to use. If the provided path does not
// exist it falls back to XDG config (~/.config/conflux/config.yaml).
func ResolveConfigPath(path string) string {
	if path == "" {
		path = "config.yaml"
	}
	if fileExists(path) {
		return path
	}
	// Only attempt fallback if original path was relative (empty or not absolute)
	if !filepath.IsAbs(path) {
		home, err := os.UserHomeDir()
		if err != nil {
			return path // can't resolve home; return original (will error later)
		}
		fallback := filepath.Join(home, ".config", "conflux", "config.yaml")
		if fileExists(fallback) {
			return fallback
		}
	}
	return path
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func Load(path string) (*Config, error) {
	resolved := ResolveConfigPath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	config.setMermaidDefaults()
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
		c.Mermaid.Width = 1200
	}
	if c.Mermaid.Height == 0 {
		c.Mermaid.Height = 800
	}
	if c.Mermaid.Scale == 0 {
		c.Mermaid.Scale = 2.0
	}
}

func (c *Config) setImageDefaults() {
	if len(c.Images.SupportedFormats) == 0 {
		c.Images.SupportedFormats = []string{"png", "jpg", "jpeg", "gif", "svg", "webp"}
	}
	if c.Images.MaxFileSize == 0 {
		c.Images.MaxFileSize = 10 * 1024 * 1024
	}
	if c.Images.MaxWidth == 0 {
		c.Images.MaxWidth = 1200
	}
	if c.Images.MaxHeight == 0 {
		c.Images.MaxHeight = 800
	}
}

func (c *Config) validateMermaid() error {
	validModes := map[string]bool{"preserve": true, "convert-to-image": true}
	if !validModes[c.Mermaid.Mode] {
		return fmt.Errorf("mermaid.mode must be 'preserve' or 'convert-to-image'")
	}
	validFormats := map[string]bool{"svg": true, "png": true, "pdf": true}
	if !validFormats[c.Mermaid.Format] {
		return fmt.Errorf("mermaid.format must be 'svg', 'png', or 'pdf'")
	}
	return nil
}

func (c *Config) validateImages() error {
	validFormats := map[string]bool{"png": true, "jpg": true, "jpeg": true, "gif": true, "svg": true, "webp": true}
	for _, format := range c.Images.SupportedFormats {
		if !validFormats[format] {
			return fmt.Errorf("images.supported_formats contains invalid format '%s', must be one of: png, jpg, jpeg, gif, svg, webp", format)
		}
	}
	if c.Images.MaxFileSize < 0 {
		return errors.New("images.max_file_size cannot be negative")
	}
	if c.Images.MaxWidth < 0 {
		return errors.New("images.max_width cannot be negative")
	}
	if c.Images.MaxHeight < 0 {
		return errors.New("images.max_height cannot be negative")
	}
	return nil
}

// LoadForSync loads config with relaxed validation for space_key (can be overridden by CLI or projects)
func LoadForSync(path string) (*Config, error) {
	resolved := ResolveConfigPath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	config.setMermaidDefaults()
	config.setImageDefaults()
	if err := config.validateForSync(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	return &config, nil
}

// LoadForListPages loads config with relaxed validation (space_key not required)
func LoadForListPages(path string) (*Config, error) {
	resolved := ResolveConfigPath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	config.setMermaidDefaults()
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
	// Global space key optional when projects present
	if len(c.Projects) == 0 && c.Confluence.SpaceKey == "" {
		return fmt.Errorf("confluence.space_key is required (or define projects with their own space_key)")
	}
	if err := c.validateProjects(); err != nil {
		return err
	}
	if err := c.validateMermaid(); err != nil {
		return err
	}
	if err := c.validateImages(); err != nil {
		return err
	}
	return nil
}

// validateForSync validates config for sync (space key can come from project or CLI)
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
	if err := c.validateProjects(); err != nil {
		return err
	}
	if err := c.validateMermaid(); err != nil {
		return err
	}
	if err := c.validateImages(); err != nil {
		return err
	}
	return nil
}

// validateForListPages validates config for list-pages (space key provided via flag or project)
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
	if err := c.validateProjects(); err != nil {
		return err
	}
	if err := c.validateMermaid(); err != nil {
		return err
	}
	if err := c.validateImages(); err != nil {
		return err
	}
	return nil
}

// validateProjects validates multi-project configuration if present
func (c *Config) validateProjects() error {
	if len(c.Projects) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	for i, p := range c.Projects {
		if p.Name == "" {
			return fmt.Errorf("projects[%d].name is required", i)
		}
		if seen[p.Name] {
			return fmt.Errorf("duplicate project name '%s'", p.Name)
		}
		seen[p.Name] = true
		if p.SpaceKey == "" {
			return fmt.Errorf("projects[%d].space_key is required", i)
		}
		if p.Local.MarkdownDir == "" {
			return fmt.Errorf("projects[%d].local.markdown_dir is required", i)
		}
	}
	return nil
}

// SelectProject applies project-specific overrides (space key + local) based on name
func (c *Config) SelectProject(name string) error {
	if name == "" {
		return fmt.Errorf("project name cannot be empty")
	}
	for _, p := range c.Projects {
		if p.Name == name {
			c.Confluence.SpaceKey = p.SpaceKey
			c.Local = p.Local
			return nil
		}
	}
	return fmt.Errorf("project '%s' not found", name)
}

// ApplyDefaultProject selects the first project if any. Returns true if applied.
func (c *Config) ApplyDefaultProject() bool {
	if len(c.Projects) == 0 {
		return false
	}
	first := c.Projects[0]
	c.Confluence.SpaceKey = first.SpaceKey
	c.Local = first.Local
	return true
}
