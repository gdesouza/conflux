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
	if c.Local.MarkdownDir == "" {
		return fmt.Errorf("local.markdown_dir is required")
	}
	return nil
}
