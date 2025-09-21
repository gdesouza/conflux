package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Confluence ConfluenceConfig `yaml:"confluence"`
	Local      LocalConfig      `yaml:"local"`
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

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
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
	return nil
}
