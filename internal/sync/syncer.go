package sync

import (
	"fmt"

	"conflux/internal/config"
	"conflux/internal/confluence"
	"conflux/internal/markdown"
	"conflux/pkg/logger"
)

type Syncer struct {
	config     *config.Config
	confluence *confluence.Client
	logger     *logger.Logger
}

func New(cfg *config.Config, log *logger.Logger) *Syncer {
	client := confluence.New(
		cfg.Confluence.BaseURL,
		cfg.Confluence.Username,
		cfg.Confluence.APIToken,
	)

	return &Syncer{
		config:     cfg,
		confluence: client,
		logger:     log,
	}
}

func (s *Syncer) Sync(dryRun bool) error {
	s.logger.Info("Starting sync process...")

	files, err := markdown.FindMarkdownFiles(s.config.Local.MarkdownDir, s.config.Local.Exclude)
	if err != nil {
		return fmt.Errorf("failed to find markdown files: %w", err)
	}

	s.logger.Info("Found %d markdown files to sync", len(files))

	for _, file := range files {
		if err := s.syncFile(file, dryRun); err != nil {
			s.logger.Error("Failed to sync file %s: %v", file, err)
			continue
		}
	}

	return nil
}

func (s *Syncer) syncFile(filePath string, dryRun bool) error {
	s.logger.Info("Processing file: %s", filePath)

	doc, err := markdown.ParseFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to parse markdown: %w", err)
	}

	confluenceContent := markdown.ConvertToConfluenceFormat(doc.Content)

	if dryRun {
		s.logger.Info("DRY RUN: Would sync page '%s'", doc.Title)
		return nil
	}

	existingPage, err := s.confluence.FindPageByTitle(s.config.Confluence.SpaceKey, doc.Title)
	if err != nil {
		return fmt.Errorf("failed to check for existing page: %w", err)
	}

	if existingPage != nil {
		s.logger.Info("Updating existing page: %s", doc.Title)
		_, err = s.confluence.UpdatePage(existingPage.ID, doc.Title, confluenceContent)
	} else {
		s.logger.Info("Creating new page: %s", doc.Title)
		_, err = s.confluence.CreatePage(s.config.Confluence.SpaceKey, doc.Title, confluenceContent)
	}

	if err != nil {
		return fmt.Errorf("failed to sync page: %w", err)
	}

	s.logger.Info("Successfully synced: %s", doc.Title)
	return nil
}
