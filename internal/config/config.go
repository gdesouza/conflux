package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Confluence ConfluenceConfig `yaml:"confluence"`
	Projects   []ProjectConfig  `yaml:"projects,omitempty"`
}

type ConfluenceConfig struct {
	BaseURL  string `yaml:"base_url"`
	Username string `yaml:"username"`
	APIToken string `yaml:"api_token"`
	SpaceKey string `yaml:"space_key"` // Optional when using projects
}

type ProjectConfig struct {
	Name     string `yaml:"name"`
	SpaceKey string `yaml:"space_key"`
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
	return load(path, true, false)
}

// LoadRuntime loads configuration for commands whose space can be selected at
// runtime by a flag, profile, environment variable, or configured default.
func LoadRuntime(path string) (*Config, error) {
	return load(path, false, true)
}

func load(path string, requireConfiguredSpace, applyEnvironment bool) (*Config, error) {
	resolved := ResolveConfigPath(path)
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if applyEnvironment {
		config.applyEnvironment()
	}

	if err := config.validateCommon(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	if requireConfiguredSpace && len(config.Projects) == 0 && config.Confluence.SpaceKey == "" {
		return nil, fmt.Errorf("invalid config: confluence.space_key is required (or define projects with their own space_key)")
	}

	return &config, nil
}

func (c *Config) applyEnvironment() {
	overrides := []struct {
		name   string
		target *string
	}{
		{name: "CONFLUX_BASE_URL", target: &c.Confluence.BaseURL},
		{name: "CONFLUX_USERNAME", target: &c.Confluence.Username},
		{name: "CONFLUX_API_TOKEN", target: &c.Confluence.APIToken},
	}
	for _, override := range overrides {
		if value := strings.TrimSpace(os.Getenv(override.name)); value != "" {
			*override.target = value
		}
	}
}

// LoadForListPages loads config with relaxed validation (space_key not required)
func LoadForListPages(path string) (*Config, error) {
	return LoadRuntime(path)
}

func (c *Config) validateCommon() error {
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
	}
	return nil
}
