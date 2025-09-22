package mermaid

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"conflux/internal/config"
	"conflux/pkg/logger"
)

type Processor struct {
	config *config.MermaidConfig
	logger *logger.Logger
}

type ProcessResult struct {
	ImagePath   string
	ImageFormat string
	Filename    string
}

func NewProcessor(config *config.MermaidConfig, logger *logger.Logger) *Processor {
	return &Processor{
		config: config,
		logger: logger,
	}
}

// CheckDependencies validates that required tools are available
func (p *Processor) CheckDependencies() error {
	if p.config.Mode == "preserve" {
		return nil // No dependencies needed for preserve mode
	}

	return p.checkCLIAvailable()
}

func (p *Processor) ProcessDiagram(diagramContent string) (*ProcessResult, error) {
	if p.config.Mode == "preserve" {
		return nil, nil // No processing needed for preserve mode
	}

	// Check if mermaid CLI is available
	if err := p.checkCLIAvailable(); err != nil {
		return nil, fmt.Errorf("mermaid CLI not available: %w", err)
	}

	// Create temporary input file
	inputFile, err := p.createTempMermaidFile(diagramContent)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp input file: %w", err)
	}
	defer os.Remove(inputFile)

	// Generate output filename
	outputFile, err := p.generateOutputFilename(diagramContent)
	if err != nil {
		return nil, fmt.Errorf("failed to generate output filename: %w", err)
	}

	// Execute mermaid CLI
	if err := p.executeMermaidCLI(inputFile, outputFile); err != nil {
		return nil, fmt.Errorf("failed to execute mermaid CLI: %w", err)
	}

	// Verify output file was created
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("output file was not created: %s", outputFile)
	}

	result := &ProcessResult{
		ImagePath:   outputFile,
		ImageFormat: p.config.Format,
		Filename:    filepath.Base(outputFile),
	}

	if p.logger != nil {
		p.logger.Debug("Successfully processed mermaid diagram to %s", outputFile)
	}

	return result, nil
}

func (p *Processor) checkCLIAvailable() error {
	_, err := exec.LookPath(p.config.CLIPath)
	if err != nil {
		return fmt.Errorf("mermaid CLI '%s' not found in PATH", p.config.CLIPath)
	}
	return nil
}

func (p *Processor) createTempMermaidFile(content string) (string, error) {
	// Create temp directory if it doesn't exist
	tempDir := filepath.Join(os.TempDir(), "conflux-mermaid")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate hash-based filename to avoid collisions
	hash := sha256.Sum256([]byte(content))
	filename := fmt.Sprintf("diagram-%x.mmd", hash)
	filePath := filepath.Join(tempDir, filename)

	// Write content to file
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	return filePath, nil
}

func (p *Processor) generateOutputFilename(content string) (string, error) {
	// Create temp directory if it doesn't exist
	tempDir := filepath.Join(os.TempDir(), "conflux-mermaid")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate hash-based filename to avoid collisions
	hash := sha256.Sum256([]byte(content))
	filename := fmt.Sprintf("diagram-%x.%s", hash, p.config.Format)
	filePath := filepath.Join(tempDir, filename)

	return filePath, nil
}

func (p *Processor) executeMermaidCLI(inputFile, outputFile string) error {
	args := []string{
		"-i", inputFile,
		"-o", outputFile,
	}

	// Add theme if specified and not default
	if p.config.Theme != "" && p.config.Theme != "default" {
		args = append(args, "-t", p.config.Theme)
	}

	cmd := exec.Command(p.config.CLIPath, args...)

	// Capture both stdout and stderr
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if p.logger != nil {
		p.logger.Debug("Executing mermaid CLI: %s %s", p.config.CLIPath, strings.Join(args, " "))
	}

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("mermaid CLI failed: %w\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	return nil
}

func (p *Processor) Cleanup(result *ProcessResult) error {
	if result == nil || result.ImagePath == "" {
		return nil
	}

	if err := os.Remove(result.ImagePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup temp file %s: %w", result.ImagePath, err)
	}

	if p.logger != nil {
		p.logger.Debug("Cleaned up temp file: %s", result.ImagePath)
	}

	return nil
}

// ValidateContent performs basic validation on mermaid diagram content
func ValidateContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("mermaid diagram content cannot be empty")
	}

	// Basic check for common mermaid diagram types
	content = strings.TrimSpace(content)
	validStarters := []string{
		"graph",
		"flowchart",
		"sequenceDiagram",
		"classDiagram",
		"stateDiagram",
		"erDiagram",
		"journey",
		"gantt",
		"pie",
		"gitgraph",
		"mindmap",
		"timeline",
		"sankey-beta",
		"xychart-beta",
		"requirementDiagram",
	}

	for _, starter := range validStarters {
		if strings.HasPrefix(strings.ToLower(content), strings.ToLower(starter)) {
			return nil
		}
	}

	// Allow custom directives and comments
	if strings.HasPrefix(content, "%%") || strings.Contains(content, "graph") || strings.Contains(content, "flowchart") {
		return nil
	}

	return fmt.Errorf("content does not appear to be a valid mermaid diagram")
}
