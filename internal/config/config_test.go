package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
  space_key: "TEST"
local:
  markdown_dir: "./docs"
  exclude: ["*.tmp"]
mermaid:
  mode: "convert-to-image"
  format: "svg"
  cli_path: "mmdc"
  theme: "default"
`,
			expectError: false,
		},
		{
			name: "valid config with defaults",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
  space_key: "TEST"
local:
  markdown_dir: "./docs"
`,
			expectError: false,
		},
		{
			name: "missing base_url",
			configData: `
confluence:
  username: "test@example.com"
  api_token: "test_token"
  space_key: "TEST"
`,
			expectError: true,
			errorMsg:    "confluence.base_url is required",
		},
		{
			name: "missing username",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  api_token: "test_token"
  space_key: "TEST"
`,
			expectError: true,
			errorMsg:    "confluence.username is required",
		},
		{
			name: "missing api_token",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  space_key: "TEST"
`,
			expectError: true,
			errorMsg:    "confluence.api_token is required",
		},
		{
			name: "missing space_key no projects",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
`,
			expectError: true,
			errorMsg:    "confluence.space_key is required",
		},
		{
			name: "projects make space_key optional",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
projects:
  - name: core
    space_key: CORE
    local:
      markdown_dir: ./core
`,
			expectError: false,
		},
		{
			name: "project missing fields",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
projects:
  - name: bad
    local:
      markdown_dir: ./core
`,
			expectError: true,
			errorMsg:    "projects[0].space_key is required",
		},
		{
			name: "duplicate project name",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
projects:
  - name: dup
    space_key: A
    local:
      markdown_dir: ./a
  - name: dup
    space_key: B
    local:
      markdown_dir: ./b
`,
			expectError: true,
			errorMsg:    "duplicate project name",
		},
		{
			name: "invalid mermaid mode",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
  space_key: "TEST"
mermaid:
  mode: "invalid"
`,
			expectError: true,
			errorMsg:    "mermaid.mode must be 'preserve' or 'convert-to-image'",
		},
		{
			name: "invalid mermaid format",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
  space_key: "TEST"
mermaid:
  format: "invalid"
`,
			expectError: true,
			errorMsg:    "mermaid.format must be 'svg', 'png', or 'pdf'",
		},
		{
			name: "invalid yaml",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: [invalid
`,
			expectError: true,
			errorMsg:    "failed to parse config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary config file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.configData), 0600); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			config, err := Load(configPath)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if config == nil {
				t.Fatal("Config is nil")
			}

			// Verify mermaid defaults are set
			if config.Mermaid.Mode == "" {
				t.Error("Mermaid mode should have default value")
			}
			if config.Mermaid.Format == "" {
				t.Error("Mermaid format should have default value")
			}
			if config.Mermaid.CLIPath == "" {
				t.Error("Mermaid CLI path should have default value")
			}
			if config.Mermaid.Theme == "" {
				t.Error("Mermaid theme should have default value")
			}
		})
	}
}

func TestLoadForSync(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config without space_key",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
local:
  markdown_dir: "./docs"
`,
			expectError: false,
		},
		{
			name: "valid config with space_key",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
  space_key: "TEST"
local:
  markdown_dir: "./docs"
`,
			expectError: false,
		},
		{
			name: "missing base_url",
			configData: `
confluence:
  username: "test@example.com"
  api_token: "test_token"
`,
			expectError: true,
			errorMsg:    "confluence.base_url is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.configData), 0600); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			config, err := LoadForSync(configPath)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if config == nil {
				t.Fatal("Config is nil")
			}
		})
	}
}

func TestLoadForListPages(t *testing.T) {
	tests := []struct {
		name        string
		configData  string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config without space_key",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
  api_token: "test_token"
`,
			expectError: false,
		},
		{
			name: "missing api_token",
			configData: `
confluence:
  base_url: "https://example.atlassian.net"
  username: "test@example.com"
`,
			expectError: true,
			errorMsg:    "confluence.api_token is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if err := os.WriteFile(configPath, []byte(tt.configData), 0600); err != nil {
				t.Fatalf("Failed to write test config: %v", err)
			}

			config, err := LoadForListPages(configPath)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if config == nil {
				t.Fatal("Config is nil")
			}
		})
	}
}

func TestLoadFileNotFound(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("Expected error for nonexistent file")
	}
	if err.Error() != "failed to read config file: open /nonexistent/config.yaml: no such file or directory" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

func TestSetMermaidDefaults(t *testing.T) {
	config := &Config{}
	config.setMermaidDefaults()

	expectedDefaults := map[string]string{
		"Mode":    "convert-to-image",
		"Format":  "svg",
		"CLIPath": "mmdc",
		"Theme":   "default",
	}

	if config.Mermaid.Mode != expectedDefaults["Mode"] {
		t.Errorf("Expected Mode '%s', got '%s'", expectedDefaults["Mode"], config.Mermaid.Mode)
	}
	if config.Mermaid.Format != expectedDefaults["Format"] {
		t.Errorf("Expected Format '%s', got '%s'", expectedDefaults["Format"], config.Mermaid.Format)
	}
	if config.Mermaid.CLIPath != expectedDefaults["CLIPath"] {
		t.Errorf("Expected CLIPath '%s', got '%s'", expectedDefaults["CLIPath"], config.Mermaid.CLIPath)
	}
	if config.Mermaid.Theme != expectedDefaults["Theme"] {
		t.Errorf("Expected Theme '%s', got '%s'", expectedDefaults["Theme"], config.Mermaid.Theme)
	}
}

func TestSetMermaidDefaultsPartial(t *testing.T) {
	config := &Config{
		Mermaid: MermaidConfig{
			Mode:   "preserve",
			Format: "png",
		},
	}
	config.setMermaidDefaults()

	// Existing values should be preserved
	if config.Mermaid.Mode != "preserve" {
		t.Errorf("Expected Mode 'preserve', got '%s'", config.Mermaid.Mode)
	}
	if config.Mermaid.Format != "png" {
		t.Errorf("Expected Format 'png', got '%s'", config.Mermaid.Format)
	}

	// Missing values should get defaults
	if config.Mermaid.CLIPath != "mmdc" {
		t.Errorf("Expected CLIPath 'mmdc', got '%s'", config.Mermaid.CLIPath)
	}
	if config.Mermaid.Theme != "default" {
		t.Errorf("Expected Theme 'default', got '%s'", config.Mermaid.Theme)
	}
}

func TestValidateMermaid(t *testing.T) {
	tests := []struct {
		name        string
		config      MermaidConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid preserve mode",
			config: MermaidConfig{
				Mode:   "preserve",
				Format: "svg",
			},
			expectError: false,
		},
		{
			name: "valid convert-to-image mode",
			config: MermaidConfig{
				Mode:   "convert-to-image",
				Format: "png",
			},
			expectError: false,
		},
		{
			name: "invalid mode",
			config: MermaidConfig{
				Mode:   "invalid",
				Format: "svg",
			},
			expectError: true,
			errorMsg:    "mermaid.mode must be 'preserve' or 'convert-to-image'",
		},
		{
			name: "invalid format",
			config: MermaidConfig{
				Mode:   "preserve",
				Format: "invalid",
			},
			expectError: true,
			errorMsg:    "mermaid.format must be 'svg', 'png', or 'pdf'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{Mermaid: tt.config}
			err := config.validateMermaid()

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error but got none")
				}
				if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	}
}
